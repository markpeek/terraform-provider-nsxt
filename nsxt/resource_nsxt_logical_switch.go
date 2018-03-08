/* Copyright © 2017 VMware, Inc. All Rights Reserved.
   SPDX-License-Identifier: MPL-2.0 */

package nsxt

import (
	"fmt"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	api "github.com/vmware/go-vmware-nsxt"
	"github.com/vmware/go-vmware-nsxt/manager"
	"log"
	"net/http"
	"time"
)

var logicalSwitchReplicationModeValues = []string{"MTEP", "SOURCE", ""}

// formatVirtualMachinePostCloneRollbackError defines the verbose error when
// rollback fails on a post-clone virtual machine operation.
const formatLogicalSwitchRollbackError = `
WARNING:
There was an error during the creation of logical switch %s:
%s
Additionally, there was an error deleting the logical switch during rollback:
%s
The logical switch may still exist in Terraform state. If it does, the
resource will need to be tainted before trying again. For more information on
how to do this, see the following page:
https://www.terraform.io/docs/commands/taint.html
If the logical switch does not exist in state, manually delete it to try again.
`

// TODO: consider splitting this resource to overlay_ls and vlan_ls
func resourceNsxtLogicalSwitch() *schema.Resource {
	return &schema.Resource{
		Create: resourceNsxtLogicalSwitchCreate,
		Read:   resourceNsxtLogicalSwitchRead,
		Update: resourceNsxtLogicalSwitchUpdate,
		Delete: resourceNsxtLogicalSwitchDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"revision": getRevisionSchema(),
			"description": &schema.Schema{
				Type:        schema.TypeString,
				Description: "Description of this resource",
				Optional:    true,
			},
			"display_name": &schema.Schema{
				Type:        schema.TypeString,
				Description: "The display name of this resource. Defaults to ID if not set",
				Optional:    true,
				Computed:    true,
			},
			"tag":             getTagsSchema(),
			"address_binding": getAddressBindingsSchema(),
			"admin_state":     getAdminStateSchema(),
			"ip_pool_id": &schema.Schema{
				Type:        schema.TypeString,
				Description: "IP pool id that associated with a LogicalSwitch",
				Optional:    true,
			},
			"mac_pool_id": &schema.Schema{
				Type:        schema.TypeString,
				Description: "Mac pool id that associated with a LogicalSwitch",
				Optional:    true,
			},
			"replication_mode": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "Replication mode of the Logical Switch",
				Optional:     true,
				Default:      "MTEP",
				ValidateFunc: validation.StringInSlice(logicalSwitchReplicationModeValues, false),
			},
			"switching_profile_id": getSwitchingProfileIdsSchema(),
			"transport_zone_id": &schema.Schema{
				Type:        schema.TypeString,
				Description: "Id of the TransportZone to which this LogicalSwitch is associated",
				Required:    true,
				ForceNew:    true,
			},
			"vlan": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"vni": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "VNI for this LogicalSwitch",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func resourceNsxtLogicalSwitchCreate(d *schema.ResourceData, m interface{}) error {
	nsxClient := m.(*api.APIClient)
	description := d.Get("description").(string)
	displayName := d.Get("display_name").(string)
	tags := getTagsFromSchema(d)
	addressBindings := getAddressBindingsFromSchema(d)
	adminState := d.Get("admin_state").(string)
	ipPoolID := d.Get("ip_pool_id").(string)
	macPoolID := d.Get("mac_pool_id").(string)
	replicationMode := d.Get("replication_mode").(string)
	switchingProfileID := getSwitchingProfileIdsFromSchema(d)
	transportZoneID := d.Get("transport_zone_id").(string)
	vlan := int64(d.Get("vlan").(int))
	vni := int32(d.Get("vni").(int))

	logicalSwitch := manager.LogicalSwitch{
		Description:         description,
		DisplayName:         displayName,
		Tags:                tags,
		AddressBindings:     addressBindings,
		AdminState:          adminState,
		IpPoolId:            ipPoolID,
		MacPoolId:           macPoolID,
		ReplicationMode:     replicationMode,
		SwitchingProfileIds: switchingProfileID,
		TransportZoneId:     transportZoneID,
		Vlan:                vlan,
		Vni:                 vni,
	}

	logicalSwitch, resp, err := nsxClient.LogicalSwitchingApi.CreateLogicalSwitch(nsxClient.Context, logicalSwitch)

	if err != nil {
		return fmt.Errorf("Error during LogicalSwitch create: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Unexpected status returned during LogicalSwitch create: %v", resp.StatusCode)
	}

	// verifying switch realization on hypervisor
	stateConf := &resource.StateChangeConf{
		Pending: []string{"in_progress", "pending", "partial_success"},
		Target:  []string{"success"},
		Refresh: func() (interface{}, string, error) {
			state, resp, err := nsxClient.LogicalSwitchingApi.GetLogicalSwitchState(nsxClient.Context, logicalSwitch.Id)
			if err != nil {
				return nil, "", fmt.Errorf("Error while querying realization state: %v", err)
			}

			if resp.StatusCode != http.StatusOK {
				return nil, "", fmt.Errorf("Unexpected return status %d", resp.StatusCode)
			}

			if state.FailureCode != 0 {
				return nil, "", fmt.Errorf("Error in switch realization: %s", state.FailureMessage)
			}

			log.Printf("[DEBUG] Realization state: %s", state.State)
			return logicalSwitch, state.State, nil
		},
		Timeout:    d.Timeout(schema.TimeoutCreate),
		MinTimeout: 1 * time.Second,
		Delay:      1 * time.Second,
	}
	_, err = stateConf.WaitForState()
	if err != nil {
		// Realization failed - rollback & delete the switch
		log.Printf("[ERROR] Rollback switch %s creation due to unrealized state", logicalSwitch.Id)
		localVarOptionals := make(map[string]interface{})
		_, derr := nsxClient.LogicalSwitchingApi.DeleteLogicalSwitch(nsxClient.Context, logicalSwitch.Id, localVarOptionals)
		if derr != nil {
			// rollback failed
			return fm.Errorf(formatLogicalSwitchRollbackError, logicalSwitch.Id, err, derr)
		}
		return err
	}

	d.SetId(logicalSwitch.Id)

	return resourceNsxtLogicalSwitchRead(d, m)
}

func resourceNsxtLogicalSwitchRead(d *schema.ResourceData, m interface{}) error {
	nsxClient := m.(*api.APIClient)
	id := d.Id()
	if id == "" {
		return fmt.Errorf("Error obtaining logical switch id")
	}

	logicalSwitch, resp, err := nsxClient.LogicalSwitchingApi.GetLogicalSwitch(nsxClient.Context, id)
	if resp.StatusCode == http.StatusNotFound {
		log.Printf("[DEBUG] LogicalSwitch %s not found", id)
		d.SetId("")
		return nil
	}
	if err != nil {
		return fmt.Errorf("Error during LogicalSwitch read: %v", err)
	}

	d.Set("revision", logicalSwitch.Revision)
	d.Set("description", logicalSwitch.Description)
	d.Set("display_name", logicalSwitch.DisplayName)
	setTagsInSchema(d, logicalSwitch.Tags)
	setAddressBindingsInSchema(d, logicalSwitch.AddressBindings)
	d.Set("admin_state", logicalSwitch.AdminState)
	d.Set("ip_pool_id", logicalSwitch.IpPoolId)
	d.Set("mac_pool_id", logicalSwitch.MacPoolId)
	d.Set("replication_mode", logicalSwitch.ReplicationMode)
	setSwitchingProfileIdsInSchema(d, nsxClient, logicalSwitch.SwitchingProfileIds)
	d.Set("transport_zone_id", logicalSwitch.TransportZoneId)
	d.Set("vlan", logicalSwitch.Vlan)
	d.Set("vni", logicalSwitch.Vni)

	return nil
}

func resourceNsxtLogicalSwitchUpdate(d *schema.ResourceData, m interface{}) error {
	nsxClient := m.(*api.APIClient)
	id := d.Id()
	if id == "" {
		return fmt.Errorf("Error obtaining logical switch id")
	}

	description := d.Get("description").(string)
	displayName := d.Get("display_name").(string)
	tags := getTagsFromSchema(d)
	addressBindings := getAddressBindingsFromSchema(d)
	adminState := d.Get("admin_state").(string)
	ipPoolID := d.Get("ip_pool_id").(string)
	macPoolID := d.Get("mac_pool_id").(string)
	replicationMode := d.Get("replication_mode").(string)
	switchingProfileID := getSwitchingProfileIdsFromSchema(d)
	transportZoneID := d.Get("transport_zone_id").(string)
	vlan := int64(d.Get("vlan").(int))
	vni := int32(d.Get("vni").(int))
	revision := int64(d.Get("revision").(int))
	logicalSwitch := manager.LogicalSwitch{
		Description:         description,
		DisplayName:         displayName,
		Tags:                tags,
		AddressBindings:     addressBindings,
		AdminState:          adminState,
		IpPoolId:            ipPoolID,
		MacPoolId:           macPoolID,
		ReplicationMode:     replicationMode,
		SwitchingProfileIds: switchingProfileID,
		TransportZoneId:     transportZoneID,
		Vlan:                vlan,
		Vni:                 vni,
		Revision:            revision,
	}

	logicalSwitch, resp, err := nsxClient.LogicalSwitchingApi.UpdateLogicalSwitch(nsxClient.Context, id, logicalSwitch)
	if err != nil || resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("Error during LogicalSwitch update: %v", err)
	}

	return resourceNsxtLogicalSwitchRead(d, m)
}

func resourceNsxtLogicalSwitchDelete(d *schema.ResourceData, m interface{}) error {
	nsxClient := m.(*api.APIClient)
	id := d.Id()
	if id == "" {
		return fmt.Errorf("Error obtaining logical switch id")
	}

	localVarOptionals := make(map[string]interface{})
	resp, err := nsxClient.LogicalSwitchingApi.DeleteLogicalSwitch(nsxClient.Context, id, localVarOptionals)
	if err != nil {
		return fmt.Errorf("Error during LogicalSwitch delete: %v", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		log.Printf("[DEBUG] LogicalSwitch %s not found", id)
		d.SetId("")
	}
	return nil
}
