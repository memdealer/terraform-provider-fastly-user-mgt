package fastly

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccFastlyDataSourceUsers_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccFastlyDataSourceUsersConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_users.test", "id"),
					resource.TestCheckResourceAttrSet("data.fastly_users.test", "users.#"),
				),
			},
		},
	})
}

const testAccFastlyDataSourceUsersConfig = `
data "fastly_users" "test" {}
`
