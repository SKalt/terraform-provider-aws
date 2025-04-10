// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch

import (
	"cmp"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	awstypes "github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	tfjson "github.com/hashicorp/terraform-provider-aws/internal/json"
	tfslices "github.com/hashicorp/terraform-provider-aws/internal/slices"
	"github.com/hashicorp/terraform-provider-aws/names"
)

const (
	fargatePlatformVersionLatest = "LATEST"
)

func containerPropertiesSchema() *schema.Resource {
	// see https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			// simply-typed attributes
			"command": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-command
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			names.AttrExecutionRoleARN: {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-executionRoleArn
				Type:     schema.TypeString,
				Optional: true,
			},
			names.AttrInstanceType: {
				Type:     schema.TypeString,
				Optional: true,
			},
			"image": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-image
				Type:     schema.TypeString,
				Optional: true,
			},
			"job_role_arn": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-jobRoleArn
				Type:     schema.TypeString,
				Optional: true,
			},
			"memory": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-memory
				Type:     schema.TypeInt,
				Optional: true,
			},
			"privileged": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-privileged
				Type:     schema.TypeBool,
				Optional: true,
			},
			"readonly_root_filesystem": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-readonlyRootFilesystem
				Type:     schema.TypeBool,
				Optional: true,
			},
			"user": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-user
				Type:     schema.TypeString,
				Optional: true,
			},
			"vcpus": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-vcpus
				Type:     schema.TypeInt,
				Optional: true,
			},

			// block attributes

			names.AttrEnvironment: {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-environment
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrName: {
							Type:     schema.TypeString,
							Required: true,
						},
						names.AttrValue: {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"ephemeral_storage": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-ephemeralStorage
				Type:     schema.TypeList,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"size_in_gib": {
							Type:     schema.TypeInt,
							Optional: true,
						},
					},
				},
			},
			"fargate_platform_configuration": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-fargatePlatformConfiguration
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_FargatePlatformConfiguration.html
				Type:     schema.TypeList,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"platform_version": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
							Default:  "LATEST",
						},
					},
				},
			},
			"linux_parameters": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-linuxParameters
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_LinuxParameters.html
				Type:     schema.TypeList,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"devices": {
							// https://docs.aws.amazon.com/batch/latest/APIReference/API_Device.html
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"container_path": {
										Type:     schema.TypeString,
										Optional: true,
									},
									"host_path": {
										Type:     schema.TypeString,
										Required: true,
									},
									"permissions": {
										Type: schema.TypeList,
										Elem: &schema.Schema{Type: schema.TypeString},
										// TODO: validate enum values
									},
								},
							},
						},
						"init_process_enabled": {
							// https://docs.aws.amazon.com/batch/latest/APIReference/API_LinuxParameters.html#Batch-Type-LinuxParameters-initProcessEnabled
							Type:     schema.TypeBool,
							Optional: true,
						},
						"max_swap": {
							// https://docs.aws.amazon.com/batch/latest/APIReference/API_LinuxParameters.html#Batch-Type-LinuxParameters-maxSwap
							Type:     schema.TypeInt,
							Optional: true,
						},
						"shared_memory_size": {
							// https://docs.aws.amazon.com/batch/latest/APIReference/API_LinuxParameters.html#Batch-Type-LinuxParameters-sharedMemorySize
							Type:     schema.TypeInt,
							Optional: true,
						},
						"swappiness": {
							// https://docs.aws.amazon.com/batch/latest/APIReference/API_LinuxParameters.html#Batch-Type-LinuxParameters-swappiness
							Type:     schema.TypeInt,
							Optional: true,
						},
						"tmpfs": {
							// https://docs.aws.amazon.com/batch/latest/APIReference/API_LinuxParameters.html#Batch-Type-LinuxParameters-tmpfs
							// https://docs.aws.amazon.com/batch/latest/APIReference/API_Tmpfs.html
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"container_path": {
										// https://docs.aws.amazon.com/batch/latest/APIReference/API_Tmpfs.html#Batch-Type-Tmpfs-containerPath
										Type:     schema.TypeString,
										Required: true,
									},
									names.AttrSize: {
										// https://docs.aws.amazon.com/batch/latest/APIReference/API_Tmpfs.html#Batch-Type-Tmpfs-size
										Type:     schema.TypeInt,
										Required: true,
									},
									"mount_options": {
										// https://docs.aws.amazon.com/batch/latest/APIReference/API_Tmpfs.html#Batch-Type-Tmpfs-mountOptions
										Type:     schema.TypeList,
										Optional: true,
										Elem:     &schema.Schema{Type: schema.TypeString},
										// TODO: validate enum values
									},
								},
							},
						},
					},
				},
			},
			"log_configuration": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-logConfiguration
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_LogConfiguration.html
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"log_driver": {
							Type:     schema.TypeString,
							Required: true,
						},
						"options": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},

						// blocks:
						"secret_options": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Schema{
								Type: schema.TypeList,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										names.AttrName: {
											Type:     schema.TypeString,
											Required: true,
										},
										"value_from": {
											Type:     schema.TypeString,
											Required: true,
										},
									},
								},
							},
						},
					},
				},
			},
			"mount_points": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-mountPoints
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_MountPoint.html
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"container_path": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"read_only": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"source_volume": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			names.AttrNetworkConfiguration: {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-networkConfiguration
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_NetworkConfiguration.html
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"assign_public_ip": {
							Type:             schema.TypeString,
							Optional:         true,
							Computed:         true,
							ValidateDiagFunc: enum.Validate[awstypes.AssignPublicIp](),
						},
					},
				},
			},
			"resource_requirements": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-resourceRequirements
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ResourceRequirement.html
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrType: {
							Type:     schema.TypeString,
							Required: true,
						},
						names.AttrValue: {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"repository_credentials": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-repositoryCredentials
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_RepositoryCredentials.html
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"credentials_parameter": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"runtime_platform": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-runtimePlatform
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_RuntimePlatform.html
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"cpu_architecture": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"operating_system_family": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"secrets": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-secrets
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_Secret.html
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrName: {
							Type:     schema.TypeString,
							Required: true,
						},
						"value_from": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"ulimits": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-ulimits
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_Ulimit.html
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrName: {Type: schema.TypeString, Required: true},
						"hard_limit":   {Type: schema.TypeInt, Optional: true},
						"soft_limit":   {Type: schema.TypeInt, Optional: true},
					},
				},
			},
			"volumes": {
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_ContainerProperties.html#Batch-Type-ContainerProperties-volumes
				// https://docs.aws.amazon.com/batch/latest/APIReference/API_Volume.html
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						names.AttrName: {Type: schema.TypeString, Required: true},
						// blocks
						"efs_volume_configuration": {
							// https://docs.aws.amazon.com/batch/latest/APIReference/API_EFSVolumeConfiguration.html
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									names.AttrFileSystemID: {Type: schema.TypeString, Optional: true},
									"root_directory":       {Type: schema.TypeString, Optional: true},
									"transit_encryption":   {Type: schema.TypeString, Optional: true},
									// blocks
									"authorization_config": {
										// https://docs.aws.amazon.com/batch/latest/APIReference/API_EFSAuthorizationConfig.html
										Type:     schema.TypeList,
										Optional: true,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"access_point_id": {Type: schema.TypeString, Optional: true},
												"iam":             {Type: schema.TypeString, Optional: true},
											},
										},
									},
								},
							},
						},
						"host": { // see https://docs.aws.amazon.com/batch/latest/APIReference/API_Host.html
							Type:     schema.TypeList,
							MaxItems: 1,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"source_path": {Type: schema.TypeString, Optional: true},
								},
							},
						},
					},
				},
			},
		},
	}
}

type containerProperties awstypes.ContainerProperties

func (cp *containerProperties) reduce() {
	cp.sortEnvironment()

	// Prevent difference of API response that adds an empty array when not configured during the request.
	if len(cp.Command) == 0 {
		cp.Command = nil
	}

	// Remove environment variables with empty values.
	cp.Environment = tfslices.Filter(cp.Environment, func(kvp awstypes.KeyValuePair) bool {
		return aws.ToString(kvp.Value) != ""
	})

	// Prevent difference of API response that adds an empty array when not configured during the request.
	if len(cp.Environment) == 0 {
		cp.Environment = nil
	}

	// Prevent difference of API response that contains the default Fargate platform configuration.
	if cp.FargatePlatformConfiguration != nil {
		if aws.ToString(cp.FargatePlatformConfiguration.PlatformVersion) == fargatePlatformVersionLatest {
			cp.FargatePlatformConfiguration = nil
		}
	}

	if cp.LinuxParameters != nil {
		if len(cp.LinuxParameters.Devices) == 0 {
			cp.LinuxParameters.Devices = nil
		}

		for i, device := range cp.LinuxParameters.Devices {
			if len(device.Permissions) == 0 {
				cp.LinuxParameters.Devices[i].Permissions = nil
			}
		}

		if len(cp.LinuxParameters.Tmpfs) == 0 {
			cp.LinuxParameters.Tmpfs = nil
		}

		for i, tmpfs := range cp.LinuxParameters.Tmpfs {
			if len(tmpfs.MountOptions) == 0 {
				cp.LinuxParameters.Tmpfs[i].MountOptions = nil
			}
		}
	}

	// Prevent difference of API response that adds an empty array when not configured during the request.
	if cp.LogConfiguration != nil {
		if len(cp.LogConfiguration.Options) == 0 {
			cp.LogConfiguration.Options = nil
		}

		if len(cp.LogConfiguration.SecretOptions) == 0 {
			cp.LogConfiguration.SecretOptions = nil
		}
	}

	// Prevent difference of API response that adds an empty array when not configured during the request.
	if len(cp.MountPoints) == 0 {
		cp.MountPoints = nil
	}

	// Prevent difference of API response that adds an empty array when not configured during the request.
	if len(cp.ResourceRequirements) == 0 {
		cp.ResourceRequirements = nil
	}

	// Prevent difference of API response that adds an empty array when not configured during the request.
	if len(cp.Secrets) == 0 {
		cp.Secrets = nil
	}

	// Prevent difference of API response that adds an empty array when not configured during the request.
	if len(cp.Ulimits) == 0 {
		cp.Ulimits = nil
	}

	// Prevent difference of API response that adds an empty array when not configured during the request.
	if len(cp.Volumes) == 0 {
		cp.Volumes = nil
	}
}

func (cp *containerProperties) sortEnvironment() {
	// Deal with Environment objects which may be re-ordered in the API.
	slices.SortFunc(cp.Environment, func(a, b awstypes.KeyValuePair) int {
		return cmp.Compare(aws.ToString(a.Name), aws.ToString(b.Name))
	})
}

func (cp1 *containerProperties) equalTo(cp2 *containerProperties) (equivalent bool, err error) {
	var b1, b2 []byte
	cp1.reduce()
	cp2.reduce()
	b1, err = tfjson.EncodeToBytes(cp1)
	if err != nil {
		return
	}
	b2, err = tfjson.EncodeToBytes(cp2)
	if err != nil {
		return
	}
	equivalent = tfjson.EqualBytes(b1, b2)
	return
}

// FIXME: remove this. Problem is, it's currently used in the test suite.
// equivalentContainerPropertiesJSON determines equality between two Batch ContainerProperties JSON strings
func equivalentContainerPropertiesJSON(str1, str2 string) (equivalent bool, err error) {
	if str1 == "" {
		str1 = "{}"
	}

	if str2 == "" {
		str2 = "{}"
	}

	var cp1 containerProperties
	err = tfjson.DecodeFromString(str1, &cp1)
	if err != nil {
		return
	}
	cp1.reduce()
	var b1 []byte
	b1, err = tfjson.EncodeToBytes(cp1)
	if err != nil {
		return
	}

	var cp2 containerProperties
	err = tfjson.DecodeFromString(str2, &cp2)
	if err != nil {
		return
	}
	cp2.reduce()
	var b2 []byte
	b2, err = tfjson.EncodeToBytes(cp2)
	if err != nil {
		return
	}

	return tfjson.EqualBytes(b1, b2), nil
}

func expandContainerProperties(input map[string]any) awstypes.ContainerProperties {
	apiObject := awstypes.ContainerProperties{}
	if v, ok := input[names.AttrExecutionRoleARN]; ok {
		apiObject.ExecutionRoleArn = v.(*string)
	}
	if v, ok := input[names.AttrInstanceType].(string); ok {
		apiObject.InstanceType = aws.String(v)
	}
	if v, ok := input["image"].(string); ok {
		apiObject.Image = aws.String(v)
	}
	if v, ok := input["job_role_arn"].(string); ok {
		apiObject.JobRoleArn = aws.String(v)
	}
	if v, ok := input["memory"].(int32); ok {
		apiObject.Memory = aws.Int32(v) // FIXME: upgrade -> resource_requirements
	}
	if v, ok := input["privileged"].(bool); ok {
		apiObject.Privileged = aws.Bool(v)
	}
	if v, ok := input["read_only_root_filesystem"].(bool); ok {
		apiObject.ReadonlyRootFilesystem = aws.Bool(v)
	}
	if v, ok := input["user"].(string); ok {
		apiObject.User = aws.String(v)
	}
	if v, ok := input["vcpus"].(int32); ok {
		apiObject.Vcpus = aws.Int32(v) // FIXME: upgrade -> resource_requirements
	}

	if v, ok := input["command"].([]any); ok && len(v) > 0 {
		apiObject.Command = flex.ExpandStringValueList(v)
	}

	if v, ok := input["environment"]; ok {
		apiObject.Environment = expandContainerEnvironment(v)
	}
	if v, ok := input["ephemeral_storage"].([]any); ok && len(v) > 0 {
		tfMap := v[0].(map[string]interface{})
		if gib, ok := tfMap["size_in_gib"]; ok {
			apiObject.EphemeralStorage = &awstypes.EphemeralStorage{
				SizeInGiB: aws.Int32(gib.(int32)),
			}
		}
	}
	if v, ok := input["fargate_platform_configuration"].([]any); ok && len(v) > 0 {
		tfMap := v[0].(map[string]any)
		if v, ok := tfMap["platform_version"]; ok {
			apiObject.FargatePlatformConfiguration = &awstypes.FargatePlatformConfiguration{
				PlatformVersion: aws.String(v.(string)),
			}
		}
	}

	if v, ok := input["linux_parameters"].([]any); ok && len(v) > 0 {
		tfMap := v[0].(map[string]interface{})
		params := &awstypes.LinuxParameters{}
		if v, ok := tfMap["devices"].([]any); ok && len(v) > 0 {
			devices := make([]awstypes.Device, len(v))
			for i, tfMapRaw := range v {
				tfMap := tfMapRaw.(map[string]interface{})
				if w, ok := tfMap["container_path"]; ok {
					devices[i].ContainerPath = aws.String(w.(string))
				}
				if w, ok := tfMap["host_path"]; ok {
					devices[i].HostPath = aws.String(w.(string))
				}
				if w, ok := tfMap["permissions"].([]any); ok {
					permissions := make([]awstypes.DeviceCgroupPermission, len(w))
					for i, perm := range w {
						permissions[i] = awstypes.DeviceCgroupPermission(perm.(string))
					}
					devices[i].Permissions = permissions
				}
			}
		}
		if v, present := tfMap["init_process_enabled"]; present {
			params.InitProcessEnabled = aws.Bool(v.(bool))
		}
		if v, ok := tfMap["max_swap"]; ok {
			params.MaxSwap = aws.Int32(v.(int32))
		}
		if v, ok := tfMap["shared_memory_size"]; ok {
			params.SharedMemorySize = aws.Int32(v.(int32))
		}
		if v, ok := tfMap["swappiness"]; ok {
			params.Swappiness = aws.Int32(v.(int32))
		}
		if v, ok := tfMap["tmpfs"].([]any); ok && len(v) > 0 {
			params.Tmpfs = make([]awstypes.Tmpfs, len(v))
			for i, tfMapRaw := range v {
				tfMap := tfMapRaw.(map[string]any)
				if w, ok := tfMap["container_path"].(string); ok {
					params.Tmpfs[i].ContainerPath = aws.String(w)
				}
				if w, ok := tfMap[names.AttrSize].(int32); ok {
					params.Tmpfs[i].Size = aws.Int32(w)
				}
				if w, ok := tfMap["mount_options"].([]any); ok {
					params.Tmpfs[i].MountOptions = flex.ExpandStringValueList(w)
				}
			}
		}
		apiObject.LinuxParameters = params
	}
	if v, ok := input["log_configuration"].([]any); ok && len(v) > 0 {
		tfMap := v[0].(map[string]any)
		cfg := awstypes.LogConfiguration{}
		if w, ok := tfMap["log_driver"].(string); ok {
			cfg.LogDriver = awstypes.LogDriver(w) // TODO: validate enum
		}
		if w, ok := tfMap["options"].(map[string]any); ok {
			cfg.Options = make(map[string]string, len(w))
			for k, v := range w {
				cfg.Options[k] = v.(string)
			}
		}
		if rawOpts, ok := tfMap["secret_options"].([]any); ok {
			cfg.SecretOptions = make([]awstypes.Secret, len(rawOpts))
			for i, rawOpt := range rawOpts {
				opt := rawOpt.(map[string]any)
				if name, ok := opt["name"].(string); ok {
					cfg.SecretOptions[i].Name = aws.String(name)
				}
				if value, ok := opt["value_from"].(string); ok {
					cfg.SecretOptions[i].ValueFrom = aws.String(value)
				}
			}
		}

		apiObject.LogConfiguration = &cfg
	}
	if v, ok := input["mount_points"].([]any); ok {
		mounts := make([]awstypes.MountPoint, len(v))
		for i, rawMount := range v {
			tfMap := rawMount.(map[string]any)
			if containerPath, ok := tfMap["container_path"].(string); ok {
				mounts[i].ContainerPath = aws.String(containerPath)
			}
			if readOnly, ok := tfMap["read_only"].(bool); ok {
				mounts[i].ReadOnly = aws.Bool(readOnly)
			}
			if sourceVolume, ok := tfMap["source_volume"].(string); ok {
				mounts[i].SourceVolume = aws.String(sourceVolume)
			}
		}
		apiObject.MountPoints = mounts
	}
	if v, ok := input[names.AttrNetworkConfiguration].([]any); ok && len(v) > 0 {
		apiObject.NetworkConfiguration = &awstypes.NetworkConfiguration{}
		if assignIp, ok := v[0].(map[string]any)["assign_public_ip"].(string); ok {
			apiObject.NetworkConfiguration.AssignPublicIp = awstypes.AssignPublicIp(assignIp)
		}
	}
	if v, ok := input["resource_requirements"].([]any); ok && len(v) > 0 {
		reqs := make([]awstypes.ResourceRequirement, len(v))
		for i, req := range v {
			tfMap := req.(map[string]any)
			if t, ok := tfMap[names.AttrType].(string); ok {
				reqs[i].Type = awstypes.ResourceType(t)
			}
			if v, ok := tfMap[names.AttrValue].(string); ok {
				reqs[i].Value = aws.String(v)
			}
		}
		apiObject.ResourceRequirements = reqs
	}
	if v, ok := input["repository_credentials"].([]any); ok && len(v) > 0 {
		apiObject.RepositoryCredentials = &awstypes.RepositoryCredentials{
			CredentialsParameter: aws.String(v[0].(map[string]any)["credentials_parameter"].(string)),
		}
	}
	if v, ok := input["runtime_platform"].([]any); ok && len(v) > 0 {
		apiObject.RuntimePlatform = &awstypes.RuntimePlatform{}
		tfMap := v[0].(map[string]any)
		if cpuArch, ok := tfMap["cpu_architecture"].(string); ok {
			apiObject.RuntimePlatform.CpuArchitecture = aws.String(cpuArch)
		}
		if osFamily, ok := tfMap["operating_system_family"].(string); ok {
			apiObject.RuntimePlatform.OperatingSystemFamily = aws.String(osFamily)
		}
	}
	if v, ok := input["secrets"].([]any); ok && len(v) > 0 {
		secrets := make([]awstypes.Secret, len(v))
		for i, raw := range v {
			opt := raw.(map[string]any)
			if name, ok := opt["name"].(string); ok {
				secrets[i].Name = aws.String(name)
			}
			if value, ok := opt["value_from"].(string); ok {
				secrets[i].ValueFrom = aws.String(value)
			}
		}
	}

	if v, ok := input["volumes"].([]any); ok && len(v) > 0 {
		volumes := make([]awstypes.Volume, len(v))
		for i, raw := range v {
			volumeTfMap := raw.(map[string]any)
			if name, ok := volumeTfMap[names.AttrName].(string); ok {
				volumes[i].Name = aws.String(name)
			}
			if host, ok := volumeTfMap["host"].([]any); ok && len(v) > 0 {
				volumes[i].Host = &awstypes.Host{
					SourcePath: aws.String(host[0].(map[string]any)["source_path"].(string)),
				}
			}
			if efsRaw, ok := volumeTfMap["efs_volume_configuration"].([]any); ok && len(efsRaw) > 0 {
				efs := awstypes.EFSVolumeConfiguration{}
				efsTfMap := efsRaw[0].(map[string]any)
				if fsId, ok := efsTfMap[names.AttrFileSystemID].(string); ok {
					efs.FileSystemId = aws.String(fsId)
				}
				if rootDir, ok := efsTfMap["root_directory"].(string); ok {
					efs.RootDirectory = aws.String(rootDir)
				}
				if transitEncryption, ok := efsTfMap["transit_encryption"].(string); ok {
					efs.TransitEncryption = awstypes.EFSTransitEncryption(transitEncryption)
				}
				if authCfg, ok := efsTfMap["authorization_config"].([]any); ok && len(authCfg) > 0 {
					tfMap := authCfg[0].(map[string]any)
					cfg := awstypes.EFSAuthorizationConfig{}
					efs.AuthorizationConfig = &cfg
					if accessPointId, ok := tfMap["access_point_id"].(string); ok {
						cfg.AccessPointId = aws.String(accessPointId)
					}
					if iam, ok := tfMap["iam"].(string); ok && len(iam) > 0 {
						cfg.Iam = awstypes.EFSAuthorizationConfigIAM(iam)
					}
					volumes[i].EfsVolumeConfiguration.AuthorizationConfig = &cfg
				}
				volumes[i].EfsVolumeConfiguration = &efs
			}
		}
		apiObject.Volumes = volumes
	}

	return apiObject
}

func expandContainerEnvironment(input any) (env []awstypes.KeyValuePair) {
	for _, tfMapRaw := range input.(*schema.Set).List() {
		tfMap := tfMapRaw.(map[string]any)
		kv := awstypes.KeyValuePair{}
		if v, ok := tfMap[names.AttrName].(string); ok && v != "" {
			kv.Name = aws.String(v)
		}

		if v, ok := tfMap[names.AttrValue].(string); ok && v != "" {
			kv.Value = aws.String(v)
		}
		env = append(env, kv)
	}

	return
}
func flattenContainerEnvironment(input []awstypes.KeyValuePair) []any {
	var tfList []any
	for _, kv := range input {
		tfMap := map[string]any{}
		_set(tfMap, names.AttrName, kv.Name)
		_set(tfMap, names.AttrValue, kv.Value)
		tfList = append(tfList, tfMap)
	}
	return tfList
}

func _set[T any](output map[string]any, key string, input *T) {
	if input != nil {
		output[key] = *input
	}
}

func flattenContainerProperties(apiObject *awstypes.ContainerProperties) []any {
	if apiObject == nil {
		return nil
	}
	input := (*containerProperties)(apiObject)
	input.sortEnvironment()
	output := map[string]any{}

	_set(output, names.AttrExecutionRoleARN, input.ExecutionRoleArn)
	_set(output, names.AttrInstanceType, input.InstanceType)
	_set(output, "image", input.Image)
	_set(output, "job_role_arn", input.JobRoleArn)
	_set(output, "memory", input.Memory)
	_set(output, "privileged", input.Privileged)
	_set(output, "readonly_root_filesystem", input.ReadonlyRootFilesystem)
	_set(output, "user", input.User)
	_set(output, "vcpus", input.Vcpus)

	// handle blocks

	if input.Command != nil && len(input.Command) > 0 {
		output["command"] = input.Command
	}
	if input.Environment != nil && len(input.Environment) > 0 {
		output["environment"] = flattenContainerEnvironment(input.Environment)
	}
	if v := input.EphemeralStorage; v != nil {
		eph := map[string]any{}
		_set(eph, "size", v.SizeInGiB)
		output["ephemeral_storage"] = []any{eph}
	}
	if v := input.FargatePlatformConfiguration; v != nil {
		fargate := map[string]any{}
		_set(fargate, "platform_version", v.PlatformVersion)
		output["fargate_platform_configuration"] = []any{fargate}
	}
	if v := input.LinuxParameters; v != nil {
		linux := map[string]any{}
		if ds := v.Devices; ds != nil {
			arr := make([]any, len(ds))
			for i, d := range ds {
				m := map[string]any{}
				_set(m, "container_path", d.ContainerPath)
				_set(m, "host_path", d.HostPath)
				{ // set permissions
					var permissions []string = nil
					for _, p := range d.Permissions {
						permissions = append(permissions, string(p))
					}
					if permissions != nil {
						m["permissions"] = permissions
					}
				}
				arr[i] = m
			}
			output["linux_parameters"] = arr
		}
		_set(linux, "init_process_enabled", v.InitProcessEnabled)
		_set(linux, "max_swap", v.MaxSwap)
		_set(linux, "shared_memory_size", v.SharedMemorySize)
		_set(linux, "swappiness", v.Swappiness)

		if v.Tmpfs != nil {
			tmpfs := make([]map[string]any, len(v.Tmpfs))
			for i, t := range v.Tmpfs {
				_set(tmpfs[i], "container_path", t.ContainerPath)
				_set(tmpfs[i], names.AttrSize, t.Size)
				if t.MountOptions != nil {
					tmpfs[i]["mount_options"] = flex.FlattenStringyValueList(t.MountOptions)
				}
			}
			linux["tmpfs"] = tmpfs
		}
		output["linux_parameters"] = []any{linux}
	}
	if v := input.LogConfiguration; v != nil {
		cfg := map[string]any{}
		cfg["log_driver"] = string(v.LogDriver)
		cfg["options"] = flex.FlattenStringValueMap(v.Options)
		{ // set secret_options
			var secretOptions []any
			for _, v := range v.SecretOptions {
				secretOptions = append(secretOptions, map[string]any{
					names.AttrName: v.Name,
					"value_from":   v.ValueFrom,
				})
			}
			if secretOptions != nil {
				cfg["secret_options"] = secretOptions
			}
		}
		output["log_configuration"] = v
	}
	if v := input.MountPoints; v != nil {
		mounts := make([]map[string]any, len(v))
		for i, m := range v {
			_set(mounts[i], "container_path", m.ContainerPath)
			_set(mounts[i], "read_only", m.ReadOnly)
			_set(mounts[i], "source_volume", m.SourceVolume)
		}
		output["mount_points"] = mounts
	}
	if v := input.NetworkConfiguration; v != nil {
		output[names.AttrNetworkConfiguration] = map[string]any{
			"assign_public_ip": string(v.AssignPublicIp),
		}
	}
	if v := input.ResourceRequirements; v != nil {
		reqs := make([]map[string]any, len(v))
		for i, req := range v {
			_set(reqs[i], names.AttrValue, req.Value)
			reqs[i][names.AttrType] = string(req.Type)
		}
		output["resource_requirements"] = reqs
	}
	if v := input.RepositoryCredentials; v != nil {
		output["repository_credentials"] = map[string]any{
			"credentials_parameter": aws.ToString(v.CredentialsParameter),
		}
	}
	if v := input.RuntimePlatform; v != nil {
		m := map[string]any{}
		_set(m, "cpu_architecture", v.CpuArchitecture)
		_set(m, "operating_system_family", v.OperatingSystemFamily)
		output["runtime_platform"] = m
	}
	if v := input.Secrets; len(v) > 0 {
		secrets := make([]map[string]any, len(v))
		for i, secret := range v {
			_set(secrets[i], names.AttrName, secret.Name)
			_set(secrets[i], "value_from", secret.ValueFrom)
		}
		output["secrets"] = secrets
	}
	if v := input.Ulimits; len(v) > 0 {
		ulimits := make([]map[string]any, len(v))
		for i, u := range v {
			_set(ulimits[i], names.AttrName, u.Name)
			_set(ulimits[i], "hard_limit", u.HardLimit)
			_set(ulimits[i], "soft_limit", u.HardLimit)
		}
		output["ulimits"] = ulimits
	}
	if v := input.Volumes; len(v) > 0 {
		volumes := make([]map[string]any, len(v))
		for i, vol := range v {
			_set(volumes[i], names.AttrName, vol.Name)
			if e := vol.EfsVolumeConfiguration; e != nil {
				efs := map[string]any{}
				_set(efs, names.AttrFileSystemID, e.FileSystemId)
				_set(efs, "root_directory", e.RootDirectory)
				if e.TransitEncryption != "" {
					efs["transit_encryption"] = e.TransitEncryption
				}
				if auth := e.AuthorizationConfig; auth != nil {
					a := map[string]any{}
					_set(a, "access_point_id", auth.AccessPointId)
					if auth.Iam != "" {
						a["iam"] = string(auth.Iam)
					}
					efs["authorization_config"] = a
				}
				volumes[i]["efs_volume_configuration"] = efs
			}
			if h := vol.Host; h != nil {
				host := map[string]any{}
				_set(host, "source_path", h.SourcePath)
				volumes[i]["host"] = host
			}
		}
		output["volumes"] = volumes
	}

	return []any{output}
}
