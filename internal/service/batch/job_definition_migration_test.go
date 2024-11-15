// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_test

import (
	"context"
	"fmt"
	"testing"

	awstypes "github.com/aws/aws-sdk-go-v2/service/batch/types"
	sdkacctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/service/batch"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccAWSBatchJobDefinition_MigrateFromSDKToPluginFramework(t *testing.T) {
	ctx := acctest.Context(t)
	var jobDef awstypes.JobDefinition
	resourceName := "aws_batch_job_definition.test"
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(ctx, t); testAccPreCheck(ctx, t) },
		ErrorCheck:   acctest.ErrorCheck(t, names.BatchServiceID),
		CheckDestroy: testAccCheckJobDefinitionDestroy(ctx),
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"aws": {
						Source:            "hashicorp/aws",
						VersionConstraint: "5.23.0", // Ensure to use the latest provider version
					},
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckBatchJobDefinitionExists(ctx, resourceName, &jobDef),
				),
				Config: testAccBatchJobDefinitionConfig_initial(rName),
			},
			{
				Config:                   testAccBatchJobDefinitionConfig_updated(rName),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				PlanOnly:                 true,
				ExpectNonEmptyPlan:       true,
			},
			{
				Config:                   testAccBatchJobDefinitionConfig_updated(rName),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "container_properties.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "container_properties.0.memory", "1024"),
				),
			},
		},
	})
}

func testAccBatchJobDefinitionConfig_initial(rName string) string {
	return fmt.Sprintf(`
resource "aws_batch_job_definition" "test" {
  name = "%s"
  type = "container"

  container_properties = jsonencode({
    image   = "my-image"
    vcpus   = 1
    memory  = 1024
    command = ["echo", "hello"]
  })
}`, rName)
}

func testAccBatchJobDefinitionConfig_updated(rName string) string {
	return fmt.Sprintf(`
resource "aws_batch_job_definition" "test" {
  name = "%s"
  type = "container"

  container_properties {
    image   = "my-image"
    vcpus   = 1
    memory  = 1024
    command = ["echo", "world"]
  }
}`, rName)
}

func testAccCheckBatchJobDefinitionExists(ctx context.Context, resourceName string, jobDef *awstypes.JobDefinition) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// Get the resource from the Terraform state
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Batch Job Definition not found: %s", resourceName)
		}

		// Extract the job definition name from the resource state
		jobDefArn := rs.Primary.Attributes["arn"]

		conn := acctest.Provider.Meta().(*conns.AWSClient).BatchClient(ctx)
		job, err := batch.FindJobDefinitionByARN(ctx, conn, jobDefArn)

		if err != nil {
			return fmt.Errorf("failed to describe Batch Job Definition: %s", err)
		}
		jobDef = job
		return nil
	}
}
