import {z} from "zod";

export const empty = z.object({});
export const page = <T extends z.ZodTypeAny>(row: T) =>
  z.object({
    items: z.array(row),
    more: z.boolean(),
    pages: z.number().int().nonnegative().optional(),
    total: z.number().int().nonnegative().optional(),
  });
export const id = z.number().int().positive();
export const tokenCount = z.number().int().nonnegative();
export const instant = z.string().datetime({offset: true});
export const named = z.object({id, name: z.string()});

export const agreementDocSchema = z.object({id: z.string(), title: z.string(), content_md: z.string()});
export const customMenuItem = z.object({
  id: z.string(),
  label: z.string(),
  url: z.string().url(),
  page_slug: z.string(),
  icon_svg: z.string().optional(),
  sort_order: z.number(),
  hide_open_button: z.boolean().optional(),
});

export const configSchema = z.object({
  official: z.string().url(),
  site_name: z.string().optional().default("Sub2API"),
  site_subtitle: z.string().optional().default(""),
  site_logo: z.string().optional().default(""),
  doc_url: z.string().optional().default(""),
  contact_info: z.string().optional().default(""),
  home_content: z.string().optional().default(""),
  compact_home: z.boolean().optional().default(false),
  backend_mode: z.boolean().optional().default(false),
  registration: z.boolean().optional().default(false),
  email_verify: z.boolean().optional().default(false),
  promo_code: z.boolean().optional().default(false),
  password_reset: z.boolean().optional().default(false),
  passkey: z.boolean().optional().default(false),
  email_whitelist: z.array(z.string()).optional().default([]),
  api_base_url: z.string().optional(),
  hide_ccs_import_button: z.boolean().optional().default(false),
  custom_endpoints: z.array(z.object({name: z.string(), endpoint: z.string().url(), description: z.string().optional()})).optional().default([]),
  custom_menu_items: z.array(customMenuItem).optional().default([]),
  agreement: z.boolean(),
  agreement_mode: z.string().optional().default("checkbox"),
  agreement_revision: z.string().optional().default(""),
  agreement_docs: z.array(agreementDocSchema).optional().default([]),
  oauth_providers: z.array(z.string()).optional().default([]),
  oidc_provider_name: z.string().optional().default(""),
  affiliate: z.boolean(),
  model_plaza_enabled: z.boolean().default(false),
  model_plaza_require_auth: z.boolean().default(false),
  mcp_enabled: z.boolean().default(false),
  channel_monitor_enabled: z.boolean().default(false),
  invitation: z.boolean(),
  purchase_enabled: z.boolean().default(false),
  available_channels_enabled: z.boolean().default(false),
  patrol_home_visible: z.boolean().optional().default(true),
  patrol_user_visible: z.boolean().optional().default(true),
  guest_probe_enabled: z.boolean().optional().default(false),
  dev_accounts: z.array(z.object({label: z.string(), email: z.string().email(), password: z.string(), otp: z.string().optional()})).optional(),
  captcha: z.object({
    provider: z.enum(["turnstile", "tencent", "aliyun"]),
    key: z.string(),
    prefix: z.string().optional(),
    region: z.string().optional(),
  }).optional(),
});

export const setupSchema = z.object({required: z.boolean()});
export const setupCandidatesSchema = z.object({candidates: z.array(z.object({url: z.string().url(), version: z.string()}))});
export type Config = z.infer<typeof configSchema>;
