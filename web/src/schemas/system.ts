import {z} from "zod";

export const builtinPluginSettingSchema = z.object({
  key: z.string(),
  title: z.string(),
  description: z.string().optional(),
  kind: z.enum(["bool", "int", "string"]),
  default: z.unknown(),
  min: z.number().optional(),
  max: z.number().optional(),
  enum: z.array(z.string()).optional(),
});

export const builtinPluginStateSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  category: z.string(),
  enabled: z.boolean(),
  status: z.enum(["off", "active", "failed"]),
  reason: z.string().optional(),
  settings: z.array(builtinPluginSettingSchema),
  config: z.record(z.string(), z.unknown()),
});

export const extPluginViewSchema = z.object({
  id: z.string(),
  name: z.string(),
  publisher: z.string(),
  summary: z.string(),
  category: z.string(),
  license: z.string(),
  homepage: z.string().optional(),
  repo: z.string().optional(),
  version: z.string().optional(),
  permissions: z.array(z.string()).default([]),
  option_specs: z.array(z.object({
    key: z.string(),
    title: z.string(),
    placeholder: z.string().optional(),
    default: z.string().optional(),
  })).default([]),
  probes: z.array(z.object({
    id: z.string(),
    title: z.string(),
    protocol: z.string(),
    presets: z.array(z.string()).optional(),
  })).default([]),
  page: z.object({
    title: z.string(),
    file: z.string().optional(),
    url: z.string().optional(),
    mount: z.string(),
  }).nullable().optional(),
  actions: z.array(z.object({id: z.string(), target: z.string(), title: z.string()})).default([]),
  tools: z.array(z.object({name: z.string(), description: z.string()})).default([]),
  official_seed: z.boolean(),
  installed: z.boolean(),
  enabled: z.boolean(),
  ready: z.boolean(),
  status_note: z.string().optional(),
  source: z.string().optional(),
  options: z.record(z.string(), z.string()),
});

export const extMarketOverviewSchema = z.object({
  market_url: z.string(),
  plugins: z.array(extPluginViewSchema),
});

export const adminPluginsBundleSchema = z.object({
  builtin: z.array(builtinPluginStateSchema),
  market: extMarketOverviewSchema,
  upstream_enabled: z.boolean(),
});

export const mountedPluginsSchema = z.object({
  items: z.array(z.object({plugin_id: z.string(), title: z.string(), mount: z.string()})),
});
