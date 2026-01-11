package fastly

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	gofastly "github.com/fastly/go-fastly/v12/fastly"
)

func dataSourceFastlyUsers() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceFastlyUsersRead,
		Schema: map[string]*schema.Schema{
			"users": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "List of all users for the current customer account",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The ID of the user",
						},
						"login": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The email address (login) of the user",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The name of the user",
						},
						"role": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The role of the user",
						},
						"customer_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The customer ID the user belongs to",
						},
						"locked": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the user account is locked",
						},
						"two_factor_auth_enabled": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether two-factor authentication is enabled",
						},
						"limit_services": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the user has limited access to services",
						},
						"created_at": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "When the user was created",
						},
						"updated_at": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "When the user was last updated",
						},
					},
				},
			},
		},
	}
}

func dataSourceFastlyUsersRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	conn := meta.(*APIClient).conn

	// Get current user to find customer ID
	currentUser, err := conn.GetCurrentUser(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	customerID := gofastly.ToValue(currentUser.CustomerID)

	// List all users for this customer
	users, err := conn.ListCustomerUsers(ctx, &gofastly.ListCustomerUsersInput{
		CustomerID: customerID,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	result := make([]map[string]any, len(users))
	for i, u := range users {
		result[i] = map[string]any{
			"id":                      gofastly.ToValue(u.UserID),
			"login":                   gofastly.ToValue(u.Login),
			"name":                    gofastly.ToValue(u.Name),
			"role":                    gofastly.ToValue(u.Role),
			"customer_id":             gofastly.ToValue(u.CustomerID),
			"locked":                  gofastly.ToValue(u.Locked),
			"two_factor_auth_enabled": gofastly.ToValue(u.TwoFactorAuthEnabled),
			"limit_services":          gofastly.ToValue(u.LimitServices),
		}

		if u.CreatedAt != nil {
			result[i]["created_at"] = u.CreatedAt.String()
		}
		if u.UpdatedAt != nil {
			result[i]["updated_at"] = u.UpdatedAt.String()
		}
	}

	if err := d.Set("users", result); err != nil {
		return diag.FromErr(err)
	}

	// Use customer ID as the data source ID
	d.SetId(customerID)

	return nil
}



