// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	awstypes "github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	internalFlex "github.com/hashicorp/terraform-provider-aws/internal/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	"github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/tags"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// Note: this resource frequently changes its ARN with a `:${revision}` suffix.
// Thus, it's computed **without** a `UseStateForUnknown()` plan modifier, since
// using using a prior ARN in the plan would introduce a discrepancy with the post-apply ARN.
// Avoiding `UseStateForUnknown` means the ARN becomes unknown before updates, which
// breaks transparent tagging (see internal/provider/fwprovider/intercept.go).
// This means we have to eject from transparent tagging and handle tag updates ourself.
//
// not using:
// @Tags(identifierAttribute="arn")

// @Testing(importIgnore="deregister_on_new_revision")
// @FrameworkResource("aws_batch_job_definition", name="Job Definition")
func newResourceJobDefinition(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &resourceJobDefinition{}

	return r, nil
}

const (
	ResNameJobDefinition = "Job Definition"
)

type resourceJobDefinition struct {
	framework.ResourceWithConfigure
}

func (r *resourceJobDefinition) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "aws_batch_job_definition"
}

func (r *resourceJobDefinition) SchemaContainer(ctx context.Context) schema.NestedBlockObject {
	return schema.NestedBlockObject{
		Attributes: map[string]schema.Attribute{
			"command": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"execution_role_arn": schema.StringAttribute{
				Optional: true,
			},
			"image": schema.StringAttribute{
				Optional: true,
			},
			"instance_type": schema.StringAttribute{
				Optional: true,
			},
			"job_role_arn": schema.StringAttribute{
				Optional: true,
			},
			"memory": schema.Int32Attribute{
				Optional: true,
			},
			"privileged": schema.BoolAttribute{
				Optional: true,
			},
			"readonly_root_filesystem": schema.BoolAttribute{
				Optional: true,
			},
			"user": schema.StringAttribute{
				Optional: true,
			},
			"vcpus": schema.Int32Attribute{
				Optional: true,
			},
		},
		Blocks: map[string]schema.Block{
			names.AttrEnvironment: schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[keyValuePairModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Optional: true,
						},
						"value": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"ephemeral_storage": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[ephemeralStorageModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"size_in_gib": schema.Int64Attribute{
							Optional: true,
						},
					},
				},
			},
			"fargate_platform_configuration": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[fargatePlatformConfigurationModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"platform_version": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"linux_parameters": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[linuxParametersModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"devices": schema.ListAttribute{
							CustomType: fwtypes.NewListNestedObjectTypeOf[deviceModel](ctx),
							Optional:   true,
						},
						"init_process_enabled": schema.BoolAttribute{
							Optional: true,
						},
						"max_swap": schema.Int64Attribute{
							Optional: true,
						},
						"shared_memory_size": schema.Int64Attribute{
							Optional: true,
						},
						"swappiness": schema.Int64Attribute{
							Optional: true,
						},
					},
					Blocks: map[string]schema.Block{
						"tmpfs": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[tmpfsModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"container_path": schema.StringAttribute{
										Optional: true,
									},
									"size": schema.Int64Attribute{
										Optional: true,
									},
									"mount_options": schema.ListAttribute{
										ElementType: types.StringType,
										Optional:    true,
									},
								},
							},
						},
					},
				},
			},
			"log_configuration": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[logConfigurationModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"log_driver": schema.StringAttribute{
							Optional: true,
						},
						"options": schema.MapAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
					},
					Blocks: map[string]schema.Block{
						"secret_options": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[secretModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Optional: true,
									},
									"value_from": schema.StringAttribute{
										Optional: true,
									},
								},
							},
						},
					},
				},
			},
			"mount_points": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[mountPointModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"container_path": schema.StringAttribute{
							Optional: true,
						},
						"read_only": schema.BoolAttribute{
							Optional: true,
						},
						"source_volume": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"network_configuration": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[networkConfigurationModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"assign_public_ip": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Validators: []validator.String{
								enum.FrameworkValidate[awstypes.AssignPublicIp](),
							},
						},
					},
				},
			},
			"resource_requirements": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[resourceRequirementModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Optional: true,
						},
						"value": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"repository_credentials": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[repositoryCredentialsModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"credentials_parameter": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"runtime_platform": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[runtimePlatformModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"cpu_architecture": schema.StringAttribute{
							Optional: true,
						},
						"operating_system_family": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"secrets": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[secretModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Optional: true,
						},
						"value_from": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"ulimits": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[ulimitModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"hard_limit": schema.Int64Attribute{
							Optional: true,
						},
						"name": schema.StringAttribute{
							Optional: true,
						},
						"soft_limit": schema.Int64Attribute{
							Optional: true,
						},
					},
				},
			},
			"volumes": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[volumeModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Optional: true,
						},
					},
					Blocks: map[string]schema.Block{
						"efs_volume_configuration": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[efsVolumeConfigurationModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"file_system_id": schema.StringAttribute{
										Optional: true,
									},
									"root_directory": schema.StringAttribute{
										Optional: true,
									},
									"transit_encryption": schema.StringAttribute{
										Optional: true,
									},
								},
								Blocks: map[string]schema.Block{
									"authorization_config": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[efsAuthorizationConfigModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"access_point_id": schema.StringAttribute{
													Optional: true,
												},
												"iam": schema.StringAttribute{
													Optional: true,
												},
											},
										},
									},
								},
							},
						},
						"host": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[hostModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"source_path": schema.StringAttribute{
										Optional: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *resourceJobDefinition) SchemaEKSContainer(ctx context.Context) schema.NestedBlockObject {
	return schema.NestedBlockObject{
		// see https://docs.aws.amazon.com/batch/latest/APIReference/API_EksContainer.html
		Attributes: map[string]schema.Attribute{
			"args": schema.ListAttribute{
				Computed:    true,
				Optional:    true,
				ElementType: types.StringType,
			},
			"command": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"image": schema.StringAttribute{
				Optional: true,
			},
			"image_pull_policy": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(imagePullPolicy_Values()...),
				},
			},
			"name": schema.StringAttribute{
				Optional: true,
			},
		},
		Blocks: map[string]schema.Block{
			"env": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[keyValuePairModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Optional: true,
						},
						"value": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"resources": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[eksContainerResourceRequirementsModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"limits": schema.MapAttribute{
							Optional:    true,
							Computed:    true,
							ElementType: types.StringType,
						},
						"requests": schema.MapAttribute{
							Computed:    true,
							Optional:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"security_context": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[eksContainerSecurityContextModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"privileged": schema.BoolAttribute{
							Optional: true,
						},
						"run_as_user": schema.Int64Attribute{
							Optional: true,
						},
						"read_only_root_file_system": schema.BoolAttribute{
							Optional: true,
						},
						"run_as_non_root": schema.BoolAttribute{
							Optional: true,
						},
						"run_as_group": schema.Int64Attribute{
							Optional: true,
						},
					},
				},
			},
			"volume_mounts": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[eksContainerVolumeMountModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"mount_path": schema.StringAttribute{
							Optional: true,
						},
						"read_only": schema.BoolAttribute{
							Optional: true,
						},
						"name": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func (r *resourceJobDefinition) SchemaECSProperties(ctx context.Context) schema.NestedBlockObject {
	return schema.NestedBlockObject{
		Blocks: map[string]schema.Block{
			"task_properties": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[ecsTaskPropertiesModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"execution_role_arn": schema.StringAttribute{
							Optional: true,
						},
						"ipc_mode": schema.StringAttribute{
							Optional: true,
						},
						"pid_mode": schema.StringAttribute{
							Optional: true,
						},
						"platform_version": schema.StringAttribute{
							Computed: true,
							Optional: true,
						},
						"task_role_arn": schema.StringAttribute{
							Optional: true,
						},
					},
					Blocks: map[string]schema.Block{
						"containers": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[taskPropertiesContainerModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"command": schema.ListAttribute{
										Optional:    true,
										ElementType: types.StringType,
									},
									"image": schema.StringAttribute{
										Optional: true,
									},
									"essential": schema.BoolAttribute{
										Optional: true,
										Computed: true,
									},
									"name": schema.StringAttribute{
										Optional: true,
									},
									"privileged": schema.BoolAttribute{
										Optional: true,
									},
									"readonly_root_filesystem": schema.BoolAttribute{
										Optional: true,
										Computed: true,
									},
									"user": schema.StringAttribute{
										Optional: true,
									},
								},
								Blocks: map[string]schema.Block{
									"depends_on": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[taskContainerDependencyModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"condition": schema.StringAttribute{
													Optional: true,
												},
												"container_name": schema.StringAttribute{
													Optional: true,
												},
											},
										},
									},
									names.AttrEnvironment: schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[keyValuePairModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"name": schema.StringAttribute{
													Optional: true,
												},
												"value": schema.StringAttribute{
													Optional: true,
												},
											},
										},
									},
									"linux_parameters": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[linuxParametersModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"devices": schema.ListAttribute{
													CustomType: fwtypes.NewListNestedObjectTypeOf[deviceModel](ctx),
													Optional:   true,
												},
												"init_process_enabled": schema.BoolAttribute{
													Optional: true,
												},
												"max_swap": schema.Int64Attribute{
													Optional: true,
												},
												"shared_memory_size": schema.Int64Attribute{
													Optional: true,
												},
												"swappiness": schema.Int64Attribute{
													Optional: true,
												},
											},
											Blocks: map[string]schema.Block{
												"tmpfs": schema.ListNestedBlock{
													CustomType: fwtypes.NewListNestedObjectTypeOf[tmpfsModel](ctx),
													NestedObject: schema.NestedBlockObject{
														Attributes: map[string]schema.Attribute{
															"container_path": schema.StringAttribute{
																Optional: true,
															},
															"size": schema.Int64Attribute{
																Optional: true,
															},
															"mount_options": schema.ListAttribute{
																ElementType: types.StringType,
																Optional:    true,
															},
														},
													},
												},
											},
										},
									},
									"log_configuration": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[logConfigurationModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"log_driver": schema.StringAttribute{
													Optional: true,
												},
												"options": schema.MapAttribute{
													Optional:    true,
													ElementType: types.StringType,
												},
											},
											Blocks: map[string]schema.Block{
												"secret_options": schema.ListNestedBlock{
													CustomType: fwtypes.NewListNestedObjectTypeOf[secretModel](ctx),
													NestedObject: schema.NestedBlockObject{
														Attributes: map[string]schema.Attribute{
															"name": schema.StringAttribute{
																Optional: true,
															},
															"value_from": schema.StringAttribute{
																Optional: true,
															},
														},
													},
												},
											},
										},
									},
									"mount_points": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[mountPointModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"container_path": schema.StringAttribute{
													Optional: true,
												},
												"read_only": schema.BoolAttribute{
													Optional: true,
												},
												"source_volume": schema.StringAttribute{
													Optional: true,
												},
											},
										},
									},
									"repository_credentials": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[repositoryCredentialsModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"credentials_parameter": schema.StringAttribute{
													Optional: true,
												},
											},
										},
									},
									"resource_requirements": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[resourceRequirementModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"type": schema.StringAttribute{
													Optional: true,
												},
												"value": schema.StringAttribute{
													Optional: true,
												},
											},
										},
									},
									"secrets": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[secretModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"name": schema.StringAttribute{
													Optional: true,
												},
												"value_from": schema.StringAttribute{
													Optional: true,
												},
											},
										},
									},
									"ulimits": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[ulimitModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"name": schema.StringAttribute{
													Optional: true,
												},
												"hard_limit": schema.Int64Attribute{
													Optional: true,
												},
												"soft_limit": schema.Int64Attribute{
													Optional: true,
												},
											},
										},
									},
								},
							},
						},
						"ephemeral_storage": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[ephemeralStorageModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"size_in_gib": schema.Int64Attribute{
										Optional: true,
									},
								},
							},
						},
						"network_configuration": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[networkConfigurationModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"assign_public_ip": schema.StringAttribute{
										Optional: true,
										Validators: []validator.String{
											enum.FrameworkValidate[awstypes.AssignPublicIp](),
										},
									},
								},
							},
						},
						"runtime_platform": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[runtimePlatformModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"cpu_architecture": schema.StringAttribute{
										Optional: true,
									},
									"operating_system_family": schema.StringAttribute{
										Optional: true,
									},
								},
							},
						},
						"volumes": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[volumeModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Optional: true,
									},
								},
								Blocks: map[string]schema.Block{
									"host": schema.ListNestedBlock{
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"source_path": schema.StringAttribute{
													Optional: true,
												},
											},
										},
									},
									"efs_volume_configuration": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[efsVolumeConfigurationModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"file_system_id": schema.StringAttribute{
													Optional: true,
												},
												"root_directory": schema.StringAttribute{
													Optional: true,
												},
												"transit_encryption": schema.StringAttribute{
													Optional: true,
												},
												"transit_encryption_port": schema.Int64Attribute{
													Optional: true,
												},
											},
											Blocks: map[string]schema.Block{
												"authorization_config": schema.ListNestedBlock{
													CustomType: fwtypes.NewListNestedObjectTypeOf[efsAuthorizationConfigModel](ctx),
													NestedObject: schema.NestedBlockObject{
														Attributes: map[string]schema.Attribute{
															"access_point_id": schema.StringAttribute{
																Optional: true,
															},
															"iam": schema.StringAttribute{
																Optional: true,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *resourceJobDefinition) SchemaEKSProperties(ctx context.Context) schema.NestedBlockObject {
	return schema.NestedBlockObject{
		Blocks: map[string]schema.Block{
			"pod_properties": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[eksPodPropertiesModel](ctx),
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"dns_policy": schema.StringAttribute{
							Optional: true,
							Validators: []validator.String{
								stringvalidator.OneOf(dnsPolicy_Values()...),
							},
						},
						"host_network": schema.BoolAttribute{
							Optional: true,
						},
						"service_account_name": schema.StringAttribute{
							Optional: true,
						},
						"share_process_namespace": schema.BoolAttribute{
							Optional: true,
						},
					},
					Blocks: map[string]schema.Block{
						"containers": schema.ListNestedBlock{
							CustomType:   fwtypes.NewListNestedObjectTypeOf[eksContainerModel](ctx),
							NestedObject: r.SchemaEKSContainer(ctx),
						},
						"image_pull_secrets": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[eksImagePullSecrets](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Optional: true,
									},
								},
							},
						},
						"init_containers": schema.ListNestedBlock{
							CustomType:   fwtypes.NewListNestedObjectTypeOf[eksContainerModel](ctx),
							NestedObject: r.SchemaEKSContainer(ctx),
						},
						"metadata": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[eksMetadataModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"labels": schema.MapAttribute{
										Optional:    true,
										Computed:    true,
										ElementType: types.StringType,
									},
								},
							},
						},
						"volumes": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[eksVolumeModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Optional: true,
									},
								},
								Blocks: map[string]schema.Block{
									"empty_dir": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[eksEmptyDirModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"medium": schema.StringAttribute{
													Optional: true,
												},
												"size_limit": schema.StringAttribute{
													Optional: true,
												},
											},
										},
									},
									"host_path": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[eksHostPathModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"path": schema.StringAttribute{
													Optional: true,
												},
											},
										},
									},
									"secret": schema.ListNestedBlock{
										CustomType: fwtypes.NewListNestedObjectTypeOf[eksSecretModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"secret_name": schema.StringAttribute{
													Optional: true,
												},
												"optional": schema.BoolAttribute{
													Optional: true,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *resourceJobDefinition) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// The ID includes the batch job definition version, and so it changes every time the job definition is updated.
			// As a result we can't use
			// - names.AttrID:  framework.IDAttribute(),
			// - names.AttrARN: framework.ARNAttributeComputedOnly(),
			//  due to the plan modifier .UseStateForUnknown()
			names.AttrARN: schema.StringAttribute{
				Computed: true,
				// PlanModifiers: []planmodifier.String{
				// 	&funkyStringPlanModifier{},
				// },
			},
			names.AttrID: schema.StringAttribute{Computed: true},

			"arn_prefix": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"deregister_on_new_revision": schema.BoolAttribute{
				Default:  booldefault.StaticBool(true),
				Optional: true,
				Computed: true,
			},

			names.AttrName: schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9A-Za-z]{1}[0-9A-Za-z_-]{0,127}$`),
						`must be up to 128 letters (uppercase and lowercase), numbers, underscores and dashes, and must start with an alphanumeric`,
					),
				},
			},

			names.AttrParameters: schema.MapAttribute{
				CustomType:  fwtypes.MapOfStringType,
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"platform_capabilities": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(
						enum.FrameworkValidate[awstypes.PlatformCapability](),
					),
				},
			},
			names.AttrPropagateTags: schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"revision": schema.Int32Attribute{
				Computed: true,
			},
			"scheduling_priority": schema.Int32Attribute{
				Optional: true,
			},
			names.AttrTags:    tftags.TagsAttribute(),             // <-
			names.AttrTagsAll: tftags.TagsAttributeComputedOnly(), // <-

			names.AttrType: schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					enum.FrameworkValidate[awstypes.JobDefinitionType](),
				},
			},
			names.AttrTimeout: schema.ListAttribute{
				CustomType: fwtypes.NewListNestedObjectTypeOf[jobTimeoutModel](ctx),
				Optional:   true,
				ElementType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"attempt_duration_seconds": types.Int64Type,
					},
				},
			},
		},
		Blocks: map[string]schema.Block{
			"container_properties": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[containerPropertiesModel](ctx),
				Validators: []validator.List{
					listvalidator.ExactlyOneOf(
						path.MatchRoot("container_properties"),
						path.MatchRoot("ecs_properties"),
						path.MatchRoot("eks_properties"),
						path.MatchRoot("node_properties"),
					),
				},
				NestedObject: r.SchemaContainer(ctx),
			},
			"ecs_properties": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[ecsPropertiesModel](ctx),
				Validators: []validator.List{
					listvalidator.ExactlyOneOf(
						path.MatchRoot("container_properties"),
						path.MatchRoot("ecs_properties"),
						path.MatchRoot("eks_properties"),
						path.MatchRoot("node_properties"),
					),
				},
				NestedObject: r.SchemaECSProperties(ctx),
			},
			"eks_properties": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[eksPropertiesModel](ctx),
				Validators: []validator.List{
					listvalidator.ExactlyOneOf(
						path.MatchRoot("container_properties"),
						path.MatchRoot("ecs_properties"),
						path.MatchRoot("eks_properties"),
						path.MatchRoot("node_properties"),
					),
				},
				NestedObject: r.SchemaEKSProperties(ctx),
			},
			"node_properties": schema.ListNestedBlock{
				// see https://docs.aws.amazon.com/batch/latest/APIReference/API_RegisterJobDefinition.html#Batch-RegisterJobDefinition-request-nodeProperties
				// see https://docs.aws.amazon.com/batch/latest/APIReference/API_NodeProperties.html
				CustomType: fwtypes.NewListNestedObjectTypeOf[nodePropertiesModel](ctx),
				Validators: []validator.List{
					listvalidator.ExactlyOneOf(
						path.MatchRoot("container_properties"),
						path.MatchRoot("ecs_properties"),
						path.MatchRoot("eks_properties"),
						path.MatchRoot("node_properties"),
					),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"main_node": schema.Int64Attribute{
							Optional: true,
						},
						"num_nodes": schema.Int64Attribute{
							Optional: true,
						},
					},
					Blocks: map[string]schema.Block{
						"node_range_properties": schema.ListNestedBlock{
							// see https://docs.aws.amazon.com/batch/latest/APIReference/API_NodeRangeProperty.html
							CustomType: fwtypes.NewListNestedObjectTypeOf[nodeRangePropertyModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"target_nodes": schema.StringAttribute{
										Optional: true,
									},
									"instance_types": schema.ListAttribute{
										Computed:    true,
										Optional:    true,
										ElementType: types.StringType,
										Validators: []validator.List{
											// https://docs.aws.amazon.com/batch/latest/APIReference/API_NodeRangeProperty.html#:~:text=this%20list%20object%20is%20currently%20limited%20to%20one%20element.
											listvalidator.SizeAtLeast(1),
											listvalidator.SizeAtMost(1),
										},
									},
								},
								Blocks: map[string]schema.Block{
									"container": schema.ListNestedBlock{
										CustomType:   fwtypes.NewListNestedObjectTypeOf[containerPropertiesModel](ctx),
										NestedObject: r.SchemaContainer(ctx),
									},
									"eks_properties": schema.ListNestedBlock{
										CustomType:   fwtypes.NewListNestedObjectTypeOf[eksPropertiesModel](ctx),
										NestedObject: r.SchemaEKSProperties(ctx),
									},
									"ecs_properties": schema.ListNestedBlock{
										CustomType:   fwtypes.NewListNestedObjectTypeOf[ecsPropertiesModel](ctx),
										NestedObject: r.SchemaECSProperties(ctx),
									},
								},
							},
						},
					},
				},
			},
			"retry_strategy": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[retryStrategyModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"attempts": schema.Int32Attribute{
							Optional: true,
							Computed: true,
						},
					},
					Blocks: map[string]schema.Block{
						"evaluate_on_exit": schema.ListNestedBlock{
							CustomType: fwtypes.NewListNestedObjectTypeOf[evaluateOnExitModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"action": schema.StringAttribute{
										Optional: true,
										Computed: true,
									},
									"on_exit_code": schema.StringAttribute{
										Optional: true,
										Computed: true,
									},
									"on_reason": schema.StringAttribute{
										Optional: true,
										Computed: true,
									},
									"on_status_reason": schema.StringAttribute{
										Optional: true,
										Computed: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *resourceJobDefinition) readJobDefinitionIntoState(ctx context.Context, jd *awstypes.JobDefinition, state *resourceJobDefinitionModel) (resp diag.Diagnostics) {
	resp.Append(flex.Flatten(ctx, jd, state,
		flex.WithIgnoredFieldNamesAppend("TagsAll"),
		// Name and Arn are prefixed by JobDefinition
		flex.WithFieldNamePrefix("JobDefinition"),
	)...)
	if resp.HasError() {
		return resp
	}

	arn := aws.ToString(jd.JobDefinitionArn)
	revision := internalFlex.StringValueToInt32Value(
		strings.Split(arn, ":")[len(strings.Split(arn, ":"))-1],
	)

	state.ID = types.StringValue(arn)
	state.ARN = types.StringValue(arn)
	state.Revision = types.Int32Value(revision)
	state.ArnPrefix = types.StringValue(strings.TrimSuffix(arn, fmt.Sprintf(":%d", revision)))

	return resp
}

func warnAboutEmptyEnvVar(name, value *string, attributePath path.Path) (result diag.Diagnostic) {
	if aws.ToString(value) == "" {
		result = diag.NewAttributeWarningDiagnostic(attributePath,
			"Ignoring environment variable",
			fmt.Sprintf("The environment variable %q has an empty value, which is ignored by the Batch service", aws.ToString(name)))
	}
	return
}

func warnAboutEmptyEnvVars(envVars []awstypes.KeyValuePair, attributePath path.Path) (diagnostics diag.Diagnostics) {
	for _, envVar := range envVars {
		diagnostics.Append(warnAboutEmptyEnvVar(envVar.Name, envVar.Value, attributePath))
	}
	return diagnostics
}

func checkEnVarsSemanticallyEqual(input, output []awstypes.KeyValuePair) (semanticallyEqual bool) {
	outputSet := make(map[string]string, len(input)) // expect len(input) values
	for _, outputEnvVar := range output {
		name := aws.ToString(outputEnvVar.Name)
		value := aws.ToString(outputEnvVar.Value)
		// assume that the API that returned the output env vars guarantees the output env vars
		// have unique keys
		outputSet[name] = value
	}

	semanticallyEqual = true
	for _, inputEnvVar := range input {
		name := aws.ToString(inputEnvVar.Name)
		inputValue := aws.ToString(inputEnvVar.Value)
		outputValue, envVarSet := outputSet[name]

		if inputValue == "" {
			// empty-valued env vars are ignored by the upstream API, so they should be missing
			semanticallyEqual = !envVarSet
		} else {
			semanticallyEqual = envVarSet && inputValue == outputValue
		}
		if !semanticallyEqual {
			return
		}
	}
	return semanticallyEqual
}

// Ensure the env vars are in their original order and reinsert ignored empty env vars
// if necessary.
func fixEnvVars(input, output []awstypes.KeyValuePair) []awstypes.KeyValuePair {
	if checkEnVarsSemanticallyEqual(input, output) {
		return input
	} else {
		return output // let Terraform raise an inconsistency error
	}
}

func fixOutputEnvVars(input batch.RegisterJobDefinitionInput, output *awstypes.JobDefinition) {
	switch {
	case input.ContainerProperties != nil:
		output.ContainerProperties.Environment = fixEnvVars(input.ContainerProperties.Environment, output.ContainerProperties.Environment)
	case input.EcsProperties != nil:
		for i, task := range input.EcsProperties.TaskProperties {
			for j, container := range task.Containers {
				container.Environment = fixEnvVars(container.Environment, output.EcsProperties.TaskProperties[i].Containers[j].Environment)
			}
		}
	case input.EksProperties != nil:
	default:
		// nothing to do
	}
}

func (r *resourceJobDefinition) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	conn := r.Meta().BatchClient(ctx)

	var plan resourceJobDefinitionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tagsAll, diagnostics := r.getTagsAllMap(ctx, req.Plan)
	if resp.Diagnostics.Append(diagnostics...); resp.Diagnostics.HasError() {
		return
	}

	input := &batch.RegisterJobDefinitionInput{
		JobDefinitionName: plan.Name.ValueStringPointer(),
		Type:              awstypes.JobDefinitionType(plan.Type.ValueString()),
		Tags:              tagsAll,
	}
	resp.Diagnostics.Append(flex.Expand(ctx, plan, input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	switch plan.Type.ValueString() { // warn about empty environment variables
	case string(awstypes.JobDefinitionTypeContainer):
		switch {
		// note: these cases are exclusive; the exclusivity is enforced by validators in the schemas above.
		case input.ContainerProperties != nil:
			resp.Diagnostics.Append(
				warnAboutEmptyEnvVars(input.ContainerProperties.Environment, path.Root("container_properties"))...,
			)
		case input.EcsProperties != nil:
			for i, taskProps := range input.EcsProperties.TaskProperties {
				for j, container := range taskProps.Containers {
					attributePath := path.Root("ecs_properties").
						AtName("task_properties").AtListIndex(i).
						AtName("container").AtListIndex(j)
					resp.Diagnostics.Append(warnAboutEmptyEnvVars(container.Environment, attributePath)...)
				}
			}
		case input.EksProperties != nil:
		default:
			// do nothing
		}
	case string(awstypes.JobDefinitionTypeMultinode):
		if nodeProperties := input.NodeProperties; nodeProperties != nil {
			for i, prop := range nodeProperties.NodeRangeProperties {
				attributePath := path.Root("node_properties").
					AtName("node_range_properties").AtListIndex(i).
					AtName("container").
					AtName("environment")
				if container := prop.Container; container != nil {
					resp.Diagnostics.Append(warnAboutEmptyEnvVars(container.Environment, attributePath)...)
				}
			}
		}
	}

	out, err := conn.RegisterJobDefinition(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.Batch, create.ErrActionCreating, ResNameJobDefinition, plan.Name.String(), err),
			err.Error(),
		)
		return
	}
	if out == nil || out.JobDefinitionArn == nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.Batch, create.ErrActionCreating, ResNameJobDefinition, plan.Name.String(), nil),
			errors.New("empty output").Error(),
		)
		return
	}

	jd, err := findJobDefinitionByARN(ctx, conn, *out.JobDefinitionArn)
	if err != nil || jd == nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.Batch, create.ErrActionSetting, ResNameJobDefinition, plan.ID.String(), err),
			err.Error(),
		)
		return
	}
	fixOutputEnvVars(*input, jd) // infallible
	resp.Diagnostics.Append(r.readJobDefinitionIntoState(ctx, jd, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceJobDefinition) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	conn := r.Meta().BatchClient(ctx)

	var state resourceJobDefinitionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := findJobDefinitionByARN(ctx, conn, state.ID.ValueString())
	if tfresource.NotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil || out == nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.Batch, create.ErrActionReading, ResNameJobDefinition, state.ID.String(), err),
			err.Error(),
		)
		return
	}
	resp.Diagnostics.Append(r.readJobDefinitionIntoState(ctx, out, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	setTagsOut(ctx, out.Tags) // FIXME: !!!
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceJobDefinition) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	conn := r.Meta().BatchClient(ctx)

	// TIP: -- 2. Fetch the plan
	var plan, state resourceJobDefinitionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planTagsAll, ds := r.getTagsAllMap(ctx, req.Plan)
	if resp.Diagnostics.Append(ds...); resp.Diagnostics.HasError() {
		return
	}

	shouldDoFullUpdate, ds := __nonTagChange(ctx, req.State, req.Plan)
	if resp.Diagnostics.Append(ds...); resp.Diagnostics.HasError() {
		return
	}
	if shouldDoFullUpdate {
		input := &batch.RegisterJobDefinitionInput{
			JobDefinitionName: state.Name.ValueStringPointer(),
			Tags:              planTagsAll,
			Type:              awstypes.JobDefinitionType(plan.Type.ValueString()),
		}

		resp.Diagnostics.Append(flex.Expand(ctx, plan, input)...)
		if resp.Diagnostics.HasError() {
			return
		}

		if resp.Diagnostics.HasError() {
			return
		}

		out, err := conn.RegisterJobDefinition(ctx, input)
		if err != nil {
			resp.Diagnostics.AddError(
				create.ProblemStandardMessage(names.Batch, create.ErrActionCreating, ResNameJobDefinition, plan.Name.String(), err),
				err.Error(),
			)
			return
		}
		if out == nil || out.JobDefinitionArn == nil {
			resp.Diagnostics.AddError(
				create.ProblemStandardMessage(names.Batch, create.ErrActionCreating, ResNameJobDefinition, plan.Name.String(), nil),
				errors.New("empty output").Error(),
			)
			return
		}

		jd, err := findJobDefinitionByARN(ctx, conn, *out.JobDefinitionArn)
		if err != nil || jd == nil {
			resp.Diagnostics.AddError(
				create.ProblemStandardMessage(names.Batch, create.ErrActionSetting, ResNameJobDefinition, plan.ID.String(), err),
				err.Error(),
			)
			return
		}
		resp.Diagnostics.Append(r.readJobDefinitionIntoState(ctx, jd, &plan)...)

		if plan.DeregisterOnNewRevision.ValueBool() {
			tflog.Debug(ctx, fmt.Sprintf("[DEBUG] Deleting previous Batch Job Definition: %s", state.ID.ValueString()))
			_, err := conn.DeregisterJobDefinition(ctx, &batch.DeregisterJobDefinitionInput{
				JobDefinition: state.ID.ValueStringPointer(),
			})

			if err != nil {
				resp.Diagnostics.AddError(
					create.ProblemStandardMessage(names.Batch, create.ErrActionDeleting, ResNameJobDefinition, aws.ToString(out.JobDefinitionArn), nil),
					err.Error(),
				)
				return
			}
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	} else {
		stateTagsAll, ds := r.getTagsAllKv(ctx, req.State)
		if resp.Diagnostics.Append(ds...); resp.Diagnostics.HasError() {
			return
		}
		{ // delete unwanted tags
			deletedTags := []string{}
			for t := range stateTagsAll {
				if _, ok := planTagsAll[t]; !ok {
					deletedTags = append(deletedTags, t)
				}
			}
			input := batch.UntagResourceInput{
				ResourceArn: new(string),
				TagKeys:     deletedTags,
			}
			conn.UntagResource(ctx, &input)
		}
		{ // add the desired tags
			input := batch.TagResourceInput{
				ResourceArn: state.ARN.ValueStringPointer(),
				Tags:        planTagsAll,
			}
			conn.TagResource(ctx, &input)
		}
		jd, err := findJobDefinitionByARN(ctx, conn, *state.ARN.ValueStringPointer())
		if err != nil || jd == nil {
			resp.Diagnostics.AddError(
				create.ProblemStandardMessage(names.Batch, create.ErrActionSetting, ResNameJobDefinition, plan.ID.String(), err),
				err.Error(),
			)
			return
		}
		resp.Diagnostics.Append(r.readJobDefinitionIntoState(ctx, jd, &plan)...)

		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}

}

func (r *resourceJobDefinition) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	ctx = tflog.SetField(ctx, "debug-op", "Delete")

	conn := r.Meta().BatchClient(ctx)

	var state resourceJobDefinitionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := &batch.DescribeJobDefinitionsInput{
		JobDefinitionName: state.Name.ValueStringPointer(),
		Status:            aws.String(jobDefinitionStatusActive),
	}

	jds, err := findJobDefinitions(ctx, conn, input)

	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.Batch, create.ErrActionReading, ResNameJobDefinition, state.ID.String(), err),
			err.Error(),
		)
	}

	for i := range jds {
		arn := aws.ToString(jds[i].JobDefinitionArn)

		_, err := conn.DeregisterJobDefinition(ctx, &batch.DeregisterJobDefinitionInput{
			JobDefinition: aws.String(arn),
		})

		if err != nil {
			resp.Diagnostics.AddError(
				create.ProblemStandardMessage(names.Batch, create.ErrActionDeleting, ResNameJobDefinition, state.ID.String(), err),
				err.Error(),
			)
			return
		}
	}
}

func (r *resourceJobDefinition) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// ctx = tflog.SetField(ctx, "debug-op", "ImportState")
	resource.ImportStatePassthroughID(ctx, path.Root(names.AttrID), req, resp)
	// // DEBUG: it ain't this. Read always gets called and fixes the ARN.
	// var model resourceJobDefinitionModel
	// if ds := resp.State.Get(ctx, &model); ds.HasError() {
	// 	panic(ds)
	// }
	// if model.ARN.IsUnknown() {
	// 	tflog.Warn(ctx, "importState:: unknown arn")
	// } else if model.ARN.IsNull() {
	// 	tflog.Warn(ctx, "importState:: null arn")
	// }
}

type getter interface { // FIXME: rename
	Get(ctx context.Context, target interface{}) diag.Diagnostics
	GetAttribute(ctx context.Context, path path.Path, target interface{}) diag.Diagnostics
}

func __getTags[Getter getter](ctx context.Context, g Getter) (tftags.Map, diag.Diagnostics) {
	var result tftags.Map
	diagnostics := g.GetAttribute(ctx, path.Root(names.AttrTags), &result)
	return result, diagnostics
}
func __tagsMustBeFullyKnown(m tftags.Map) (diagnostics diag.Diagnostics) {
	if m.IsUnknown() {
		diagnostics.AddError("unknown tags map", "")
		return
	}

	for k, v := range m.Elements() {
		if v.IsUnknown() {
			diagnostics.AddError("unknown element encountered during construction of concrete tag map", k)
			return
		}
	}
	return
}

func (r *resourceJobDefinition) getResourceTags(ctx context.Context, val getter) (resourceTags tags.KeyValueTags, diagnostics diag.Diagnostics) {
	var planTags tftags.Map
	diagnostics.Append(val.GetAttribute(ctx, path.Root(names.AttrTags), &planTags)...)
	if diagnostics.HasError() {
		return
	}
	if diagnostics.Append(__tagsMustBeFullyKnown(planTags)...); diagnostics.HasError() {
		return
	}
	resourceTags = tftags.New(ctx, planTags)
	return
}

// func __tagsHaveChange(ctx context.Context, state tfsdk.State, plan tfsdk.Plan) (tagsHaveChange bool, diagnostics diag.Diagnostics) {
// 	var stateTags, planTags tftags.Map
// 	stateTags, diagnostics = __getTags(ctx, state)
// 	if diagnostics.HasError() {
// 		return
// 	}
// 	planTags, diagnostics = __getTags(ctx, plan)
// 	if diagnostics.HasError() {
// 		return
// 	}
// 	return false, nil
// }

func __nonTagChange(ctx context.Context, state tfsdk.State, plan tfsdk.Plan) (nonTagChangeDetected bool, diagnostics diag.Diagnostics) {
	diff, err := state.Raw.Diff(plan.Raw)
	if err != nil {
		diagnostics.AddError("failed to diff", err.Error())
		return
	}
	// HACK: use string comparison to check diffs under tags, tagsAll
	_tagsPrefix := path.Root(names.AttrTags).String()
	_tagsAllPrefix := path.Root(names.AttrTagsAll).String()
	for _, d := range diff {
		// filter out diffs in tags/tagsAll
		if !strings.HasPrefix(d.Path.String(), _tagsPrefix) && !strings.HasPrefix(d.Path.String(), _tagsAllPrefix) {
			nonTagChangeDetected = true
			return
		}
	}
	return
}

func (r *resourceJobDefinition) getTagsAllKv(ctx context.Context, val getter) (tagsAll tags.KeyValueTags, diagnostics diag.Diagnostics) {
	meta := r.Meta()
	defaultTagsConfig := meta.DefaultTagsConfig(ctx)
	ignoredTagsConfig := meta.IgnoreTagsConfig(ctx)
	var resourceTags tags.KeyValueTags
	resourceTags, diagnostics = r.getResourceTags(ctx, val)
	if diagnostics.HasError() {
		return
	}
	tagsAll = defaultTagsConfig.MergeTags(resourceTags).IgnoreConfig(ignoredTagsConfig)
	return
}
func (r *resourceJobDefinition) getTagsAllMap(ctx context.Context, val getter) (tagsAll map[string]string, diagnostics diag.Diagnostics) {
	var tagsAllKv tags.KeyValueTags
	tagsAllKv, diagnostics = r.getTagsAllKv(ctx, val)
	if diagnostics.HasError() {
		return
	}
	tagsAll = tagsAllKv.Map()
	return
}

// func (r *resourceJobDefinition) getTagsIn(ctx context.Context, plan tfsdk.Plan) (tags, tagsAll map[string]string, diagnostics diag.Diagnostics) {
// 	meta := r.Meta()
// 	ctx = meta.RegisterLogger(ctx)
// 	defaultTagsConfig := meta.DefaultTagsConfig(ctx)
// 	ignoredTagsConfig := meta.IgnoreTagsConfig(ctx)

// 	var planTags tftags.Map
// 	diagnostics.Append(plan.GetAttribute(ctx, path.Root(names.AttrTags), &planTags)...)

// 	if diagnostics.HasError() {
// 		return
// 	}
// 	if planTags.IsUnknown() {
// 		hasUnknownElements := false
// 		{ // copied from internal/framework/resource_with_configure.go func mapHasUnknownElements
// 			for _, v := range planTags.Elements() {
// 				if v.IsUnknown() {
// 					hasUnknownElements = true
// 					break
// 				}
// 			}
// 		}
// 		if !hasUnknownElements {
// 			resourceTags := tftags.New(ctx, planTags)
// 			allTags := defaultTagsConfig.MergeTags(resourceTags).IgnoreConfig(ignoredTagsConfig)

// 			diagnostics.Append(plan.SetAttribute(ctx, path.Root(names.AttrTagsAll), flex.FlattenFrameworkStringValueMapLegacy(ctx, allTags.Map()))...)
// 		} else {
// 			diagnostics.Append(plan.SetAttribute(ctx, path.Root(names.AttrTagsAll), tftags.Unknown)...)
// 		}
// 	}

// 	// model.Tags
// 	// defaultTagsConfig.MergeTags()
// 	// ctx = tftags.NewContext(ctx, , )
// 	// defaultTagsConfig := r.Meta().DefaultTagsConfig(ctx)
// 	// ignoreTagsConfig := r.Meta().IgnoreTagsConfig(ctx)
// 	return
// }
// func (r *resourceJobDefinition) getTagsOut() {}

func (r *resourceJobDefinition) ModifyPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse) {
	ctx = tflog.SetField(ctx, "debug-op", "ModifyPlan")
	r.SetTagsAll(ctx, request, response) // doesn't rely on context; should work?
	// HACK: (unsuccessfully) try setting and unsetting the ARN
	// stateArn := types.StringUnknown()
	// planArn := types.StringUnknown()
	// arnPath := path.Root(names.AttrARN)

	// if request.State.Raw.IsKnown() {
	// 	response.Diagnostics.Append(request.State.GetAttribute(ctx, arnPath, &stateArn)...)
	// }
	// if request.Plan.Raw.IsKnown() {
	// 	response.Diagnostics.Append(request.Plan.GetAttribute(ctx, arnPath, &planArn)...)
	// }

	// edgeCase := request.Plan.Raw.IsKnown() && request.State.Raw.IsKnown() && !(stateArn.IsUnknown() || stateArn.IsNull()) && (planArn.IsUnknown())
	// if edgeCase {
	// 	tflog.Debug(ctx, "edge case!!")
	// 	// temporarily use the old arn to update tags
	// 	response.Diagnostics.Append(request.Plan.SetAttribute(ctx, arnPath, stateArn)...)
	// 	if response.Diagnostics.HasError() {
	// 		return
	// 	}
	// }

	// if response.Diagnostics.HasError() {
	// 	return
	// }

	// if edgeCase {
	// 	// restore the unknown value of the incoming arn
	// 	response.Diagnostics.Append(response.Plan.SetAttribute(ctx, arnPath, types.StringUnknown())...)
	// }
}

type resourceJobDefinitionModel struct {
	ARN                     types.String                                              `tfsdk:"arn"`
	ArnPrefix               types.String                                              `tfsdk:"arn_prefix" autoflex:"-"`
	ContainerProperties     fwtypes.ListNestedObjectValueOf[containerPropertiesModel] `tfsdk:"container_properties"`
	DeregisterOnNewRevision types.Bool                                                `tfsdk:"deregister_on_new_revision" autoflex:"-"`
	ECSProperties           fwtypes.ListNestedObjectValueOf[ecsPropertiesModel]       `tfsdk:"ecs_properties"`
	EKSProperties           fwtypes.ListNestedObjectValueOf[eksPropertiesModel]       `tfsdk:"eks_properties"`
	ID                      types.String                                              `tfsdk:"id" autoflex:"-"`
	Name                    types.String                                              `tfsdk:"name"`
	NodeProperties          fwtypes.ListNestedObjectValueOf[nodePropertiesModel]      `tfsdk:"node_properties"`
	Parameters              fwtypes.MapOfString                                       `tfsdk:"parameters"`
	PlatformCapabilities    types.Set                                                 `tfsdk:"platform_capabilities"`
	PropagateTags           types.Bool                                                `tfsdk:"propagate_tags" autoflex:",legacy"`
	Revision                types.Int32                                               `tfsdk:"revision"`
	RetryStrategy           fwtypes.ListNestedObjectValueOf[retryStrategyModel]       `tfsdk:"retry_strategy"`
	SchedulingPriority      types.Int32                                               `tfsdk:"scheduling_priority"`
	Tags                    tftags.Map                                                `tfsdk:"tags"`
	TagsAll                 tftags.Map                                                `tfsdk:"tags_all"`
	Timeout                 fwtypes.ListNestedObjectValueOf[jobTimeoutModel]          `tfsdk:"timeout"`
	Type                    types.String                                              `tfsdk:"type"`
}

// Helper Functions
func findJobDefinitionByARN(ctx context.Context, conn *batch.Client, arn string) (*awstypes.JobDefinition, error) {
	const (
		jobDefinitionStatusInactive = "INACTIVE"
	)
	input := &batch.DescribeJobDefinitionsInput{
		JobDefinitions: []string{arn},
	}

	output, err := findJobDefinition(ctx, conn, input)

	if err != nil {
		return nil, err
	}

	if status := aws.ToString(output.Status); status == jobDefinitionStatusInactive {
		return nil, &retry.NotFoundError{
			Message:     status,
			LastRequest: input,
		}
	}

	return output, nil
}

func findJobDefinition(ctx context.Context, conn *batch.Client, input *batch.DescribeJobDefinitionsInput) (*awstypes.JobDefinition, error) {
	output, err := findJobDefinitions(ctx, conn, input)

	if err != nil {
		return nil, err
	}

	return tfresource.AssertSingleValueResult(output)
}

func findJobDefinitions(ctx context.Context, conn *batch.Client, input *batch.DescribeJobDefinitionsInput) ([]awstypes.JobDefinition, error) {
	var output []awstypes.JobDefinition

	pages := batch.NewDescribeJobDefinitionsPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)

		if err != nil {
			return nil, err
		}

		output = append(output, page.JobDefinitions...)
	}

	return output, nil
}

type ecsPropertiesModel struct {
	TaskProperties fwtypes.ListNestedObjectValueOf[ecsTaskPropertiesModel] `tfsdk:"task_properties"`
}

type ecsTaskPropertiesModel struct {
	Containers           fwtypes.ListNestedObjectValueOf[taskPropertiesContainerModel] `tfsdk:"containers"`
	EphemeralStorage     fwtypes.ListNestedObjectValueOf[ephemeralStorageModel]        `tfsdk:"ephemeral_storage"`
	ExecutionRoleArn     types.String                                                  `tfsdk:"execution_role_arn"`
	IPCMode              types.String                                                  `tfsdk:"ipc_mode"`
	NetworkConfiguration fwtypes.ListNestedObjectValueOf[networkConfigurationModel]    `tfsdk:"network_configuration"`
	PidMode              types.String                                                  `tfsdk:"pid_mode"`
	PlatformVersion      types.String                                                  `tfsdk:"platform_version"`
	RuntimePlatform      fwtypes.ListNestedObjectValueOf[runtimePlatformModel]         `tfsdk:"runtime_platform"`
	TaskRoleArn          types.String                                                  `tfsdk:"task_role_arn"`
	Volumes              fwtypes.ListNestedObjectValueOf[volumeModel]                  `tfsdk:"volumes"`
}

type repositoryCredentialsModel struct {
	CredentialsParameter types.String `tfsdk:"credentials_parameter"`
}
