package fastly

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	gofastly "github.com/fastly/go-fastly/v12/fastly"
)

const fastlyUser = "fastly_user.foo"

// TestAccFastlyUser_invitation tests the invitation workflow.
// Since invitations require user acceptance, this test verifies:
// 1. An invitation is created successfully
// 2. The resource tracks the invitation_id
// 3. The invitation can be destroyed
func TestAccFastlyUser_invitation(t *testing.T) {
	login := fmt.Sprintf("tf-test-%s@example.com", acctest.RandString(10))
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	role := "engineer"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckUserOrInvitationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig(login, name, role),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckFastlyInvitationExists(),
					resource.TestCheckResourceAttr(
						fastlyUser, "login", login),
					resource.TestCheckResourceAttr(
						fastlyUser, "name", name),
					resource.TestCheckResourceAttr(
						fastlyUser, "role", role),
					// Verify invitation_id is set (invitation is pending)
					resource.TestCheckResourceAttrSet(
						fastlyUser, "invitation_id"),
					// Verify user_id is empty (not yet accepted)
					resource.TestCheckResourceAttr(
						fastlyUser, "user_id", ""),
				),
			},
		},
	})
}

// TestAccFastlyUser_existingUser tests that if a user already exists,
// the resource correctly adopts them instead of creating an invitation.
func TestAccFastlyUser_existingUser(t *testing.T) {
	// This test would require a pre-existing user which isn't practical
	// in automated testing. Skip for now.
	t.Skip("Skipping: requires pre-existing user")
}

// testAccCheckFastlyInvitationExists verifies that either an invitation
// or a user exists for the resource.
func testAccCheckFastlyInvitationExists() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[fastlyUser]
		if !ok {
			return fmt.Errorf("not found: %s", fastlyUser)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no User/Invitation ID is set")
		}

		// Check if we have an invitation_id or user_id
		invitationID := rs.Primary.Attributes["invitation_id"]
		userID := rs.Primary.Attributes["user_id"]

		if invitationID == "" && userID == "" {
			return fmt.Errorf("neither invitation_id nor user_id is set")
		}

		conn := testAccProvider.Meta().(*APIClient).conn

		// If we have a user_id, verify the user exists
		if userID != "" {
			_, err := conn.GetUser(context.TODO(), &gofastly.GetUserInput{
				UserID: userID,
			})
			if err != nil {
				return fmt.Errorf("error getting user %s: %s", userID, err)
			}
			return nil
		}

		// If we have an invitation_id, verify the invitation exists
		if invitationID != "" {
			client := testAccProvider.Meta().(*APIClient)
			_, err := getInvitation(context.TODO(), client, invitationID)
			if err != nil {
				return fmt.Errorf("error getting invitation %s: %s", invitationID, err)
			}
			return nil
		}

		return nil
	}
}

// testAccCheckFastlyUserExists verifies that a user exists.
// This is kept for backward compatibility but may fail if invitation is pending.
func testAccCheckFastlyUserExists(user *gofastly.User) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[fastlyUser]
		if !ok {
			return fmt.Errorf("not found: %s", fastlyUser)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no User ID is set")
		}

		// Check if this is an invitation (user hasn't accepted yet)
		userID := rs.Primary.Attributes["user_id"]
		if userID == "" {
			// Still pending invitation - this is valid but user doesn't exist yet
			return nil
		}

		conn := testAccProvider.Meta().(*APIClient).conn
		latest, err := conn.GetUser(context.TODO(), &gofastly.GetUserInput{
			UserID: userID,
		})
		if err != nil {
			return err
		}

		*user = *latest

		return nil
	}
}

// testAccCheckUserOrInvitationDestroy verifies that both users and invitations
// are properly cleaned up.
func testAccCheckUserOrInvitationDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_user" {
			continue
		}

		conn := testAccProvider.Meta().(*APIClient).conn

		// Check if there's a user_id to verify user deletion
		userID := rs.Primary.Attributes["user_id"]
		if userID != "" {
			u, err := conn.GetCurrentUser(context.TODO())
			if err != nil {
				return fmt.Errorf("error getting current user when checking destroy: %s", err)
			}

			l, err := conn.ListCustomerUsers(context.TODO(), &gofastly.ListCustomerUsersInput{
				CustomerID: gofastly.ToValue(u.CustomerID),
			})
			if err != nil {
				return fmt.Errorf("error listing users when checking destroy: %s", err)
			}

			for _, u := range l {
				if gofastly.ToValue(u.UserID) == userID {
					return fmt.Errorf("user (%s) still exists after destroy", userID)
				}
			}
		}

		// Check if there's an invitation_id to verify invitation deletion
		invitationID := rs.Primary.Attributes["invitation_id"]
		if invitationID != "" {
			client := testAccProvider.Meta().(*APIClient)
			_, err := getInvitation(context.TODO(), client, invitationID)
			if err == nil {
				return fmt.Errorf("invitation (%s) still exists after destroy", invitationID)
			}
			// Error is expected (invitation should not be found)
		}
	}
	return nil
}

// testAccCheckUserDestroy is kept for backward compatibility
func testAccCheckUserDestroy(s *terraform.State) error {
	return testAccCheckUserOrInvitationDestroy(s)
}

func testAccUserConfig(login, name, role string) string {
	return fmt.Sprintf(`
resource "fastly_user" "foo" {
	login = "%s"
	name  = "%s"
	role  = "%s"
}`, login, name, role)
}
