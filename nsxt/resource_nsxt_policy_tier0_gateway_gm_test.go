/* Copyright © 2019 VMware, Inc. All Rights Reserved.
   SPDX-License-Identifier: MPL-2.0 */

package nsxt

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"testing"
)

var testAccGmGatewayIntersiteSubnet = "10.10.2.0/24"

// NOTE: This test assumes single edge cluster on both sites
func TestAccResourceNsxtPolicyTier0Gateway_globalManager(t *testing.T) {
	testResourceName := "nsxt_policy_tier0_gateway.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccEnvDefined(t, "NSXT_TEST_SITE_NAME")
			testAccEnvDefined(t, "NSXT_TEST_ANOTHER_SITE_NAME")
		},
		Providers: testAccProviders,
		CheckDestroy: func(state *terraform.State) error {
			return testAccNsxtPolicyTier0CheckDestroy(state, defaultTestResourceName)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccNsxtPolicyTier0GMCreateTemplate(true),
				Check: resource.ComposeTestCheckFunc(
					testAccNsxtPolicyTier0Exists(testResourceName),
					resource.TestCheckResourceAttr(testResourceName, "display_name", defaultTestResourceName),
					resource.TestCheckResourceAttr(testResourceName, "ha_mode", "ACTIVE_ACTIVE"),
					resource.TestCheckResourceAttr(testResourceName, "locale_service.#", "1"),
					resource.TestCheckResourceAttr(testResourceName, "intersite_config.#", "1"),
					resource.TestCheckResourceAttr(testResourceName, "intersite_config.0.transit_subnet", testAccGmGatewayIntersiteSubnet),
					resource.TestCheckResourceAttrSet(testResourceName, "intersite_config.0.primary_site_path"),
					resource.TestCheckResourceAttrSet(testResourceName, "path"),
					resource.TestCheckResourceAttrSet(testResourceName, "revision"),
				),
			},
			{
				Config: testAccNsxtPolicyTier0GMUpdateTemplate(),
				Check: resource.ComposeTestCheckFunc(
					testAccNsxtPolicyTier0Exists(testResourceName),
					resource.TestCheckResourceAttr(testResourceName, "display_name", defaultTestResourceName),
					resource.TestCheckResourceAttr(testResourceName, "ha_mode", "ACTIVE_ACTIVE"),
					resource.TestCheckResourceAttr(testResourceName, "locale_service.#", "2"),
					resource.TestCheckResourceAttr(testResourceName, "intersite_config.#", "1"),
					resource.TestCheckResourceAttr(testResourceName, "intersite_config.0.transit_subnet", testAccGmGatewayIntersiteSubnet),
					resource.TestCheckResourceAttrSet(testResourceName, "intersite_config.0.primary_site_path"),
					resource.TestCheckResourceAttrSet(testResourceName, "path"),
					resource.TestCheckResourceAttrSet(testResourceName, "revision"),
				),
			},
			{
				Config: testAccNsxtPolicyTier0GMMinimalistic(),
				Check: resource.ComposeTestCheckFunc(
					testAccNsxtPolicyTier0Exists(testResourceName),
					resource.TestCheckResourceAttr(testResourceName, "display_name", defaultTestResourceName),
					resource.TestCheckResourceAttr(testResourceName, "ha_mode", "ACTIVE_ACTIVE"),
					resource.TestCheckResourceAttr(testResourceName, "locale_service.#", "0"),
					resource.TestCheckResourceAttr(testResourceName, "intersite_config.#", "1"),
					resource.TestCheckResourceAttrSet(testResourceName, "path"),
					resource.TestCheckResourceAttrSet(testResourceName, "revision"),
				),
			},
		},
	})
}

// NOTE: This test assumes single edge cluster on both sites
func TestAccResourceNsxtPolicyTier0Gateway_globalManagerNoSubnet(t *testing.T) {
	testResourceName := "nsxt_policy_tier0_gateway.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccEnvDefined(t, "NSXT_TEST_SITE_NAME")
			testAccEnvDefined(t, "NSXT_TEST_ANOTHER_SITE_NAME")
		},
		Providers: testAccProviders,
		CheckDestroy: func(state *terraform.State) error {
			return testAccNsxtPolicyTier0CheckDestroy(state, defaultTestResourceName)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccNsxtPolicyTier0GMCreateTemplate(false),
				Check: resource.ComposeTestCheckFunc(
					testAccNsxtPolicyTier0Exists(testResourceName),
					resource.TestCheckResourceAttr(testResourceName, "display_name", defaultTestResourceName),
					resource.TestCheckResourceAttr(testResourceName, "locale_service.#", "1"),
					resource.TestCheckResourceAttr(testResourceName, "intersite_config.#", "1"),
					resource.TestCheckResourceAttrSet(testResourceName, "intersite_config.0.transit_subnet"),
					resource.TestCheckResourceAttrSet(testResourceName, "intersite_config.0.primary_site_path"),
					resource.TestCheckResourceAttrSet(testResourceName, "path"),
					resource.TestCheckResourceAttrSet(testResourceName, "revision"),
				),
			},
			{
				Config: testAccNsxtPolicyTier0GMUpdateTemplate(),
				Check: resource.ComposeTestCheckFunc(
					testAccNsxtPolicyTier0Exists(testResourceName),
					resource.TestCheckResourceAttr(testResourceName, "display_name", defaultTestResourceName),
					resource.TestCheckResourceAttr(testResourceName, "locale_service.#", "2"),
					resource.TestCheckResourceAttr(testResourceName, "intersite_config.#", "1"),
					resource.TestCheckResourceAttr(testResourceName, "intersite_config.0.transit_subnet", testAccGmGatewayIntersiteSubnet),
					resource.TestCheckResourceAttrSet(testResourceName, "intersite_config.0.primary_site_path"),
					resource.TestCheckResourceAttrSet(testResourceName, "path"),
					resource.TestCheckResourceAttrSet(testResourceName, "revision"),
				),
			},
		},
	})
}


// TODO: Add test for ACTIVE_ACTIVE when HA VIP config is supported

func testAccNsxtPolicyGMGatewayDeps() string {
	return fmt.Sprintf(`
data "nsxt_policy_site" "site1" {
  display_name = "%s"
}

data "nsxt_policy_site" "site2" {
  display_name = "%s"
}

data "nsxt_policy_edge_cluster" "ec_site1" {
  site_path = data.nsxt_policy_site.site1.path
}

data "nsxt_policy_edge_cluster" "ec_site2" {
  site_path = data.nsxt_policy_site.site2.path
}

data "nsxt_policy_edge_node" "en_site1" {
  edge_cluster_path = data.nsxt_policy_edge_cluster.ec_site1.path
  member_index      = 0
}`, getTestSiteName(), getTestAnotherSiteName())
}

func testAccNsxtPolicyTier0GMCreateTemplate(withSubnet bool) string {

	subnet := ""
	if withSubnet {
		subnet = fmt.Sprintf(`transit_subnet = "%s"`, testAccGmGatewayIntersiteSubnet)
	}
	return testAccNsxtPolicyGMGatewayDeps() + fmt.Sprintf(`
resource "nsxt_policy_tier0_gateway" "test" {
  display_name = "%s"
  ha_mode      = "ACTIVE_ACTIVE"

  locale_service {
    edge_cluster_path    = data.nsxt_policy_edge_cluster.ec_site1.path
    preferred_edge_paths = [data.nsxt_policy_edge_node.en_site1.path]
  }

  intersite_config {
    primary_site_path = data.nsxt_policy_site.site1.path
    fallback_site_paths = [data.nsxt_policy_site.site2.path]
    %s
  }
}`, defaultTestResourceName, subnet)
}

func testAccNsxtPolicyTier0GMUpdateTemplate() string {
	return testAccNsxtPolicyGMGatewayDeps() + fmt.Sprintf(`
resource "nsxt_policy_tier0_gateway" "test" {
  display_name = "%s"
  ha_mode      = "ACTIVE_ACTIVE"

  locale_service {
    edge_cluster_path    = data.nsxt_policy_edge_cluster.ec_site1.path
    preferred_edge_paths = [data.nsxt_policy_edge_node.en_site1.path]
  }

  locale_service {
    edge_cluster_path = data.nsxt_policy_edge_cluster.ec_site2.path
  }

  intersite_config {
    primary_site_path = data.nsxt_policy_site.site2.path
    transit_subnet    = "%s"
  }
}`, defaultTestResourceName, testAccGmGatewayIntersiteSubnet)
}

func testAccNsxtPolicyTier0GMMinimalistic() string {
	return testAccNsxtPolicyGMGatewayDeps() + fmt.Sprintf(`
resource "nsxt_policy_tier0_gateway" "test" {
  display_name = "%s"
}`, defaultTestResourceName)
}