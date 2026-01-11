package fastly

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	gofastly "github.com/fastly/go-fastly/v12/fastly"

	"github.com/fastly/terraform-provider-fastly-user-mgt/version"
)

// TerraformProviderProductUserAgent is included in the User-Agent header for
// any API requests made by the provider.
const TerraformProviderProductUserAgent = "terraform-provider-fastly-user-mgt"

// This value can be set to allow terraform output to display sensitive info.
var DisplaySensitiveFields = false

// Provider returns a *schema.Provider for Fastly User Management.
func Provider() *schema.Provider {
	DisplaySensitiveFields = os.Getenv("FASTLY_TF_DISPLAY_SENSITIVE_FIELDS") == "true"

	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("FASTLY_API_KEY", nil),
				Description: "Fastly API Key from https://app.fastly.com/#account",
			},
			"base_url": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("FASTLY_API_URL", gofastly.DefaultEndpoint),
				Description: "Fastly API URL",
			},
			"force_http2": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Set this to `true` to disable HTTP/1.x fallback mechanism that the underlying Go library will attempt upon connection to `api.fastly.com:443` by default. This may slightly improve the provider's performance and reduce unnecessary TLS handshakes. Default: `false`",
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"fastly_users":       dataSourceFastlyUsers(),
			"fastly_invitations": dataSourceFastlyInvitations(),
		},
		ResourcesMap: map[string]*schema.Resource{
			"fastly_user": resourceUser(),
		},
	}

	provider.ConfigureContextFunc = func(ctx context.Context, d *schema.ResourceData) (any, diag.Diagnostics) {
		config := Config{
			APIKey:     d.Get("api_key").(string),
			BaseURL:    d.Get("base_url").(string),
			ForceHTTP2: d.Get("force_http2").(bool),
			NoAuth:     false, // User management always requires auth
			UserAgent:  provider.UserAgent(TerraformProviderProductUserAgent, version.ProviderVersion),
			Context:    ctx,
		}
		return config.Client()
	}

	return provider
}
