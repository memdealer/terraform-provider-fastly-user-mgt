package fastly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	gofastly "github.com/fastly/go-fastly/v12/fastly"
)

// Invitation API types for JSON:API format
type invitationRequest struct {
	Data invitationData `json:"data"`
}

type invitationData struct {
	Type          string                  `json:"type"`
	Attributes    invitationAttributes    `json:"attributes"`
	Relationships invitationRelationships `json:"relationships"`
}

type invitationAttributes struct {
	Email         string   `json:"email"`
	LimitServices bool     `json:"limit_services"`
	Role          string   `json:"role,omitempty"`
	Roles         []string `json:"roles,omitempty"`
}

type invitationRelationships struct {
	Customer customerRelationship `json:"customer"`
}

type customerRelationship struct {
	Data customerData `json:"data"`
}

type customerData struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type invitationResponseData struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Email      string `json:"email"`
		Role       string `json:"role"`
		StatusCode int    `json:"status_code"`
	} `json:"attributes"`
}

type invitationResponse struct {
	Data invitationResponseData `json:"data"`
}

type listInvitationsResponse struct {
	Data []invitationResponseData `json:"data"`
}

func resourceUser() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceUserCreate,
		ReadContext:   resourceUserRead,
		UpdateContext: resourceUserUpdate,
		DeleteContext: resourceUserDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"login": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The email address, which is the login name, of the User",
			},

			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The real life name of the user",
			},

			"role": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "user",
				Description:      "The role of this user. Can be `user` (the default), `billing`, `engineer`, or `superuser`. For detailed information on the abilities granted to each role, see [Fastly's Documentation on User roles](https://docs.fastly.com/en/guides/configuring-user-roles-and-permissions#user-roles-and-what-they-can-do)",
				ValidateDiagFunc: validateUserRole(),
			},

			// Internal state to track pending invitation
			"invitation_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID of the pending invitation (only set while invitation is pending)",
			},

			// Tracks if user has been created (invitation accepted)
			"user_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The actual user ID (set once invitation is accepted)",
			},
		},
	}
}

func resourceUserCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*APIClient)
	conn := client.conn
	login := d.Get("login").(string)
	role := d.Get("role").(string)

	// First, check if user already exists (e.g., was invited outside of Terraform)
	existingUser, err := findUserByLogin(ctx, conn, login)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error checking for existing user: %w", err))
	}

	if existingUser != nil {
		// User already exists, just import them
		d.SetId(gofastly.ToValue(existingUser.UserID))
		if err := d.Set("user_id", gofastly.ToValue(existingUser.UserID)); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("invitation_id", ""); err != nil {
			return diag.FromErr(err)
		}
		return resourceUserRead(ctx, d, meta)
	}

	// Check if there's already a pending invitation for this email
	existingInvitation, err := findInvitationByEmail(ctx, client, login)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error checking for existing invitation: %w", err))
	}

	if existingInvitation != nil {
		// Invitation already exists, track it
		d.SetId(existingInvitation.ID)
		if err := d.Set("invitation_id", existingInvitation.ID); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("user_id", ""); err != nil {
			return diag.FromErr(err)
		}
		log.Printf("[DEBUG] Found existing invitation for %s: %s", login, existingInvitation.ID)
		return nil
	}

	// No existing user or invitation - create a new invitation
	currentUser, err := conn.GetCurrentUser(ctx)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error getting current user: %w", err))
	}

	customerID := gofastly.ToValue(currentUser.CustomerID)

	invitation, err := createInvitation(ctx, client, login, role, customerID)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating invitation: %w", err))
	}

	// Set the invitation ID as the resource ID initially
	d.SetId(invitation.Data.ID)
	if err := d.Set("invitation_id", invitation.Data.ID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("user_id", ""); err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[DEBUG] Created invitation for %s: %s", login, invitation.Data.ID)

	return nil
}

func resourceUserRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	log.Printf("[DEBUG] Refreshing User Configuration for (%s)", d.Id())
	client := meta.(*APIClient)
	conn := client.conn

	userID := d.Get("user_id").(string)
	invitationID := d.Get("invitation_id").(string)
	login := d.Get("login").(string)

	// If we have a user_id, the invitation was accepted - read the user
	if userID != "" {
		u, err := conn.GetUser(ctx, &gofastly.GetUserInput{
			UserID: userID,
		})
		if err != nil {
			// User might have been deleted
			if httpErr, ok := err.(*gofastly.HTTPError); ok && httpErr.IsNotFound() {
				d.SetId("")
				return nil
			}
			return diag.FromErr(err)
		}

		if u.Login != nil {
			if err := d.Set("login", u.Login); err != nil {
				return diag.FromErr(err)
			}
		}
		if u.Name != nil {
			if err := d.Set("name", u.Name); err != nil {
				return diag.FromErr(err)
			}
		}
		if u.Role != nil {
			if err := d.Set("role", u.Role); err != nil {
				return diag.FromErr(err)
			}
		}
		return nil
	}

	// If we have an invitation_id, check if the invitation is still pending
	// or if the user has accepted it
	if invitationID != "" {
		// First, check if user now exists (invitation was accepted)
		existingUser, err := findUserByLogin(ctx, conn, login)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error checking for user: %w", err))
		}

		if existingUser != nil {
			// User has accepted the invitation! Update state to reflect this
			newUserID := gofastly.ToValue(existingUser.UserID)
			d.SetId(newUserID)
			if err := d.Set("user_id", newUserID); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("invitation_id", ""); err != nil {
				return diag.FromErr(err)
			}

			// Read the rest of user attributes
			if existingUser.Login != nil {
				if err := d.Set("login", existingUser.Login); err != nil {
					return diag.FromErr(err)
				}
			}
			if existingUser.Name != nil {
				if err := d.Set("name", existingUser.Name); err != nil {
					return diag.FromErr(err)
				}
			}
			if existingUser.Role != nil {
				if err := d.Set("role", existingUser.Role); err != nil {
					return diag.FromErr(err)
				}
			}

			log.Printf("[DEBUG] User %s accepted invitation, transitioning to user_id %s", login, newUserID)
			return nil
		}

		// Check if the invitation still exists
		invitation, err := getInvitation(ctx, client, invitationID)
		if err != nil {
			// Invitation might have been deleted or expired
			// This is not an error - just means we need to recreate it
			log.Printf("[DEBUG] Invitation %s no longer exists, will recreate on next apply", invitationID)
			d.SetId("")
			return nil
		}

		// Invitation still pending - this is fine, keep the state as-is
		log.Printf("[DEBUG] Invitation %s still pending for %s (status_code: %d)",
			invitationID, login, invitation.StatusCode)
		return nil
	}

	// No user_id or invitation_id - this might be an import by user ID
	// Try to read as a user first
	u, err := conn.GetUser(ctx, &gofastly.GetUserInput{
		UserID: d.Id(),
	})
	if err != nil {
		if httpErr, ok := err.(*gofastly.HTTPError); ok && httpErr.IsNotFound() {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	// Successfully read as a user - update state accordingly
	if err := d.Set("user_id", d.Id()); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("invitation_id", ""); err != nil {
		return diag.FromErr(err)
	}

	if u.Login != nil {
		if err := d.Set("login", u.Login); err != nil {
			return diag.FromErr(err)
		}
	}
	if u.Name != nil {
		if err := d.Set("name", u.Name); err != nil {
			return diag.FromErr(err)
		}
	}
	if u.Role != nil {
		if err := d.Set("role", u.Role); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceUserUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	conn := meta.(*APIClient).conn

	userID := d.Get("user_id").(string)

	// Can only update if the user exists (invitation has been accepted)
	if userID == "" {
		// Invitation is still pending - we can't update name/role yet
		// The role was set in the invitation, so changes would need to
		// delete and recreate the invitation
		if d.HasChanges("name", "role") {
			return diag.Errorf("cannot update user while invitation is still pending; please wait for the user to accept the invitation")
		}
		return nil
	}

	// Update Name and/or Role.
	if d.HasChanges("name", "role") {
		_, err := conn.UpdateUser(ctx, &gofastly.UpdateUserInput{
			UserID: userID,
			Name:   gofastly.ToPointer(d.Get("name").(string)),
			Role:   gofastly.ToPointer(d.Get("role").(string)),
		})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return resourceUserRead(ctx, d, meta)
}

func resourceUserDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*APIClient)
	conn := client.conn

	userID := d.Get("user_id").(string)
	invitationID := d.Get("invitation_id").(string)

	// If there's a user, delete the user
	if userID != "" {
		err := conn.DeleteUser(ctx, &gofastly.DeleteUserInput{
			UserID: userID,
		})
		if err != nil {
			// Ignore not found errors
			if httpErr, ok := err.(*gofastly.HTTPError); ok && httpErr.IsNotFound() {
				return nil
			}
			return diag.FromErr(err)
		}
		return nil
	}

	// If there's a pending invitation, delete it
	if invitationID != "" {
		err := deleteInvitation(ctx, client, invitationID)
		if err != nil {
			// Ignore not found errors - invitation might have expired
			log.Printf("[DEBUG] Error deleting invitation %s (may have already expired): %v", invitationID, err)
		}
		return nil
	}

	return nil
}

// Helper function to find a user by their login email
func findUserByLogin(ctx context.Context, conn *gofastly.Client, login string) (*gofastly.User, error) {
	currentUser, err := conn.GetCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	users, err := conn.ListCustomerUsers(ctx, &gofastly.ListCustomerUsersInput{
		CustomerID: gofastly.ToValue(currentUser.CustomerID),
	})
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if gofastly.ToValue(u.Login) == login {
			return u, nil
		}
	}

	return nil, nil
}

// Invitation represents a pending invitation
type Invitation struct {
	ID         string
	Email      string
	Role       string
	StatusCode int
}

// Helper function to find an invitation by email
func findInvitationByEmail(ctx context.Context, client *APIClient, email string) (*Invitation, error) {
	invitations, err := listInvitations(ctx, client)
	if err != nil {
		return nil, err
	}

	for _, inv := range invitations.Data {
		if inv.Attributes.Email == email {
			return &Invitation{
				ID:         inv.ID,
				Email:      inv.Attributes.Email,
				Role:       inv.Attributes.Role,
				StatusCode: inv.Attributes.StatusCode,
			}, nil
		}
	}

	return nil, nil
}

// API functions for invitations (using raw HTTP since go-fastly may not have these)

func createInvitation(ctx context.Context, client *APIClient, email, role, customerID string) (*invitationResponse, error) {
	reqBody := invitationRequest{
		Data: invitationData{
			Type: "invitation",
			Attributes: invitationAttributes{
				Email:         email,
				LimitServices: false,
				Role:          role,
			},
			Relationships: invitationRelationships{
				Customer: customerRelationship{
					Data: customerData{
						ID:   customerID,
						Type: "customer",
					},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := doInvitationRequest(ctx, client, "POST", "/invitations", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create invitation: %s - %s", resp.Status, string(respBody))
	}

	var result invitationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func getInvitation(ctx context.Context, client *APIClient, invitationID string) (*Invitation, error) {
	// List all invitations and find the one we want
	// (The API doesn't have a direct GET for a single invitation)
	invitations, err := listInvitations(ctx, client)
	if err != nil {
		return nil, err
	}

	for _, inv := range invitations.Data {
		if inv.ID == invitationID {
			return &Invitation{
				ID:         inv.ID,
				Email:      inv.Attributes.Email,
				Role:       inv.Attributes.Role,
				StatusCode: inv.Attributes.StatusCode,
			}, nil
		}
	}

	return nil, fmt.Errorf("invitation not found: %s", invitationID)
}

func listInvitations(ctx context.Context, client *APIClient) (*listInvitationsResponse, error) {
	resp, err := doInvitationRequest(ctx, client, "GET", "/invitations", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list invitations: %s - %s", resp.Status, string(respBody))
	}

	var result listInvitationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func deleteInvitation(ctx context.Context, client *APIClient, invitationID string) error {
	resp, err := doInvitationRequest(ctx, client, "DELETE", "/invitations/"+invitationID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete invitation: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

func doInvitationRequest(ctx context.Context, client *APIClient, method, path string, body []byte) (*http.Response, error) {
	// Build the request URL using the client's base URL
	url := client.conn.Address + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set required headers for JSON:API
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Fastly-Key", client.apiKey)

	return client.conn.HTTPClient.Do(req)
}
