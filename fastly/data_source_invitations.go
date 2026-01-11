package fastly

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	gofastly "github.com/fastly/go-fastly/v12/fastly"
)

func dataSourceFastlyInvitations() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceFastlyInvitationsRead,
		Schema: map[string]*schema.Schema{
			"invitations": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "List of all pending invitations for the current customer account",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The ID of the invitation",
						},
						"email": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The email address of the invitee",
						},
						"role": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The role assigned to the invitee",
						},
						"status_code": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The status code of the invitation",
						},
					},
				},
			},
		},
	}
}

func dataSourceFastlyInvitationsRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*APIClient)
	conn := client.conn

	// Get current user to find customer ID (for the data source ID)
	currentUser, err := conn.GetCurrentUser(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	customerID := gofastly.ToValue(currentUser.CustomerID)

	// List all invitations
	invitations, err := listInvitations(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	result := make([]map[string]any, len(invitations.Data))
	for i, inv := range invitations.Data {
		result[i] = map[string]any{
			"id":          inv.ID,
			"email":       inv.Attributes.Email,
			"role":        inv.Attributes.Role,
			"status_code": inv.Attributes.StatusCode,
		}
	}

	if err := d.Set("invitations", result); err != nil {
		return diag.FromErr(err)
	}

	// Use customer ID as the data source ID
	d.SetId(customerID)

	return nil
}



