package sentry

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/jianyuan/go-sentry/sentry"
)

func resourceSentryProject() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSentryProjectCreate,
		ReadContext:   resourceSentryProjectRead,
		UpdateContext: resourceSentryProjectUpdate,
		DeleteContext: resourceSentryProjectDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceSentryProjectImporter,
		},

		Schema: map[string]*schema.Schema{
			"organization": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The slug of the organization the project belongs to",
			},
			"team": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The slug of the team to create the project for",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name for the project",
			},
			"slug": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The optional slug for this project",
				Computed:    true,
			},
			"platform": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The optional platform for this project",
				Computed:    true,
			},
			"project_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"is_public": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"is_bookmarked": {
				Type:       schema.TypeBool,
				Computed:   true,
				Deprecated: "is_bookmarked is no longer used",
			},
			"color": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"features": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"digests_min_delay": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The minimum amount of time (in seconds) to wait between scheduling digests for delivery after the initial scheduling.",
				Optional:    true,
			},
			"digests_max_delay": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The maximum amount of time (in seconds) to wait between scheduling digests for delivery.",
				Optional:    true,
			},
			"resolve_age": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Hours in which an issue is automatically resolve if not seen after this amount of time.",
				Computed:    true,
			},
            "allowed_domains": {
                Type:        schema.TypeList,
                Computed:    true,
                Description: "The domains which Sentry will allow errors to be reported from",
                Optional:    true,
                Elem: &schema.Schema{
                    Type: schema.TypeString,
                },
            },
			"remove_default_key": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to remove the default key",
				Default:     false,
			},
			"remove_default_rule": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to remove the default rule",
				Default:     false,
			},

			// TODO: Project options
		},
	}
}

func resourceSentryProjectCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*sentry.Client)

	org := d.Get("organization").(string)
	team := d.Get("team").(string)
	params := &sentry.CreateProjectParams{
		Name: d.Get("name").(string),
		Slug: d.Get("slug").(string),
	}

	tflog.Debug(ctx, "Creating Sentry project", "teamName", team, "org", org)
	proj, _, err := client.Projects.Create(org, team, params)
	if err != nil {
		return diag.FromErr(err)
	}
	tflog.Debug(ctx, "Created Sentry project", "projectSlug", proj.Slug, "projectID", proj.ID, "team", team, "org", org)

    if d.Get("remove_default_key").(bool) {
        err = removeDefaultKey(client, org, proj.Slug)
        if err != nil {
            return diag.FromErr(err)
        }
    }

    if d.Get("remove_default_rule").(bool) {
        err = removeDefaultRule(client, org, proj.Slug)
        if err != nil {
            return diag.FromErr(err)
        }
    }

	d.SetId(proj.Slug)
	return resourceSentryProjectUpdate(ctx, d, meta)
}

func resourceSentryProjectRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*sentry.Client)

	slug := d.Id()
	org := d.Get("organization").(string)

	tflog.Debug(ctx, "Reading Sentry project", "projectSlug", slug, "org", org)
	proj, resp, err := client.Projects.Get(org, slug)
	if found, err := checkClientGet(resp, err, d); !found {
		return diag.FromErr(err)
	}
	tflog.Debug(ctx, "Read Sentry project", "projectSlug", proj.Slug, "projectID", proj.ID, "org", org)

	d.SetId(proj.Slug)
	d.Set("organization", proj.Organization.Slug)
	d.Set("team", proj.Team.Slug)
	d.Set("name", proj.Name)
	d.Set("slug", proj.Slug)
	d.Set("platform", proj.Platform)
	d.Set("project_id", proj.ID)
	d.Set("is_public", proj.IsPublic)
	d.Set("color", proj.Color)
	d.Set("features", proj.Features)
	d.Set("status", proj.Status)
	d.Set("digests_min_delay", proj.DigestsMinDelay)
	d.Set("digests_max_delay", proj.DigestsMaxDelay)
	d.Set("resolve_age", proj.ResolveAge)
	d.Set("allowed_domains", proj.AllowedDomains)

	// TODO: Project options

	return nil
}

func resourceSentryProjectUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*sentry.Client)

	slug := d.Id()
	org := d.Get("organization").(string)
	params := &sentry.UpdateProjectParams{
		Name: d.Get("name").(string),
		Slug: d.Get("slug").(string),
	}

	platform := d.Get("platform").(string)
	if platform != "" {
		params.Platform = platform
	}

	if v, ok := d.GetOk("digests_min_delay"); ok {
		params.DigestsMinDelay = Int(v.(int))
	}

	if v, ok := d.GetOk("digests_max_delay"); ok {
		params.DigestsMaxDelay = Int(v.(int))
	}

	if v, ok := d.GetOk("resolve_age"); ok {
		params.ResolveAge = Int(v.(int))
	}

    allowedDomains := []string{}
    for _, url := range d.Get("allowed_domains").([]interface{}) {
        allowedDomains = append(allowedDomains, url.(string))
    }
    if len(allowedDomains) > 0 {
        params.AllowedDomains = allowedDomains
    }

	tflog.Debug(ctx, "Updating Sentry project", "projectSlug", slug, "org", org)
	proj, _, err := client.Projects.Update(org, slug, params)
	if err != nil {
		return diag.FromErr(err)
	}
	tflog.Debug(ctx, "Updated Sentry project", "projectSlug", proj.Slug, "projectID", proj.ID, "org", org)

	d.SetId(proj.Slug)
	return resourceSentryProjectRead(ctx, d, meta)
}

func resourceSentryProjectDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*sentry.Client)

	slug := d.Id()
	org := d.Get("organization").(string)

	tflog.Debug(ctx, "Deleting Sentry project", "projectSlug", slug, "org", org)
	_, err := client.Projects.Delete(org, slug)
	tflog.Debug(ctx, "Deleted Sentry project", "projectSlug", slug, "org", org)

	return diag.FromErr(err)
}

func resourceSentryProjectImporter(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	addrID := d.Id()

	tflog.Debug(ctx, "Importing Sentry project", "projetID", addrID)

	parts := strings.Split(addrID, "/")

	if len(parts) != 2 {
		return nil, errors.New("Project import requires an ADDR ID of the following schema org-slug/project-slug")
	}

	d.Set("organization", parts[0])
	d.SetId(parts[1])

	return []*schema.ResourceData{d}, nil
}

func removeDefaultKey(client *sentry.Client, org, projSlug string) error {
	keys, _, err := client.ProjectKeys.List(org, projSlug)
	if err != nil {
		return err
	}
	var defaultKeyID string
	for _, key := range keys {
		if key.Name == "Default" {
			defaultKeyID = key.ID
			break
		}
	}

	client.ProjectKeys.Delete(org, projSlug, defaultKeyID)
	return nil
}

func removeDefaultRule(client *sentry.Client, org, projSlug string) error {
	rules, _, err := client.Rules.List(org, projSlug)
	if err != nil {
		return err
	}
	var defaultRuleID string
	for _, rule := range rules {
		if rule.Name == "Send a notification for new issues" {
			defaultRuleID = rule.ID
			break
		}
	}

	client.Rules.Delete(org, projSlug, defaultRuleID)
	return nil
}
