function normalizedBase(value: string) {
  return value.replace(/\/+$/, "");
}

function encodeFragment(values: Record<string, string>) {
  return new URLSearchParams(values).toString();
}

export function magpieImportLink(input: {name: string; baseUrl: string; platform: string; key: string}) {
  const base = normalizedBase(input.baseUrl);
  const parameters: Record<string, string> = {
    name: input.name,
    key: input.key,
    website: base,
    keys: `${base}/dashboard/keys`,
  };
  if (input.platform === "openai" || input.platform === "grok") {
    parameters.chat = `${base}/v1`;
    parameters.responses = `${base}/v1`;
  } else if (input.platform === "gemini") {
    parameters.gemini = `${base}/v1beta`;
  } else {
    parameters.anthropic = input.platform === "antigravity" ? `${base}/antigravity` : base;
  }
  return `https://usemagpie.ai/import#${encodeFragment(parameters)}`;
}

export function ccSwitchImportLink(input: {name: string; baseUrl: string; platform: string; key: string}) {
  const base = normalizedBase(input.baseUrl);
  let app = "claude";
  let endpoint = base;
  let model = "";
  if (input.platform === "openai") {
    app = "codex";
    model = "gpt-6.1-sol";
  } else if (input.platform === "gemini") {
    app = "gemini";
  } else if (input.platform === "grok") {
    app = "grokbuild";
    model = "grok-4.5";
    if (!endpoint.endsWith("/v1")) endpoint += "/v1";
  } else if (input.platform === "antigravity") {
    endpoint += "/antigravity";
  }
  const usageScript = `({request:{url:"{{baseUrl}}".replace(/\\/+$/,"").replace(/\\/v1$/,"")+"/v1/usage",method:"GET",headers:{Authorization:"Bearer {{apiKey}}"}},extractor:function(response){return{isValid:response?.is_active??true,remaining:response?.remaining??response?.quota?.remaining,unit:response?.unit??"USD"}}})`;
  const entries: [string, string][] = [
    ...(model ? [["model", model] as [string, string]] : []),
    ["resource", "provider"],
    ["app", app],
    ["name", input.name],
    ["homepage", base],
    ["endpoint", endpoint],
    ["apiKey", input.key],
    ["configFormat", "json"],
    ["usageEnabled", "true"],
    ["usageScript", btoa(usageScript)],
    ["usageAutoInterval", "30"],
  ];
  const values: Record<string, string> = Object.fromEntries(entries);
  return `ccswitch://v1/import?${encodeFragment(values)}`;
}
