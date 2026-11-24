import {useMemo, useState} from "react";
import {Plus, Trash2} from "lucide-react";
import {Badge} from "../primitives/badge";
import {Button} from "../primitives/button";
import {Checkbox} from "../primitives/checkbox";
import {PLATFORM_OPTIONS} from "../domain/entity-select";
import {Field, FieldBoundary} from "./field";
import {SearchInput} from "./filter-bar";
import {FormGrid} from "./form";
import {Input} from "../primitives/input";
import {InsetRow} from "./inset-group";
import {InlineStack, Stack} from "../primitives/layout";
import {MultiGroupSelect} from "../domain/plan-picker";
import {Select} from "../primitives/select";
import {Switch} from "../primitives/switch";

export type PermissionEntry = {
  id: string;
  label: string;
  category: string;
  scope: string;
};

export type ScopedAccessRule = {
  permissions: string[];
  group_ids: number[];
  all_groups: boolean;
  include_ungrouped: boolean;
  account_types: string[];
  platforms: string[];
};

const accountTypeOptions = [
  ["oauth", "OAuth"],
  ["apikey", "API Key"],
  ["setup-token", "Setup Token"],
  ["upstream", "上游转发"],
  ["bedrock", "Bedrock"],
  ["service_account", "服务账号"],
] as const;

export function PermissionPicker({
  permissions,
  value,
  scopeFilter,
  onChange,
}: {
  permissions: readonly PermissionEntry[];
  value: readonly string[];
  scopeFilter?: "global" | "scoped";
  onChange: (next: string[]) => void;
}) {
  const [keyword, setKeyword] = useState("");
  const q = keyword.trim().toLowerCase();

  const filtered = useMemo(() => {
    return permissions.filter(entry => {
      if (scopeFilter === "global" && entry.scope && entry.scope !== "global") return false;
      if (scopeFilter === "scoped" && entry.scope === "global") return false;
      if (!q) return true;
      return entry.label.toLowerCase().includes(q) || entry.id.toLowerCase().includes(q) || entry.category.toLowerCase().includes(q);
    });
  }, [permissions, q, scopeFilter]);

  const categories = useMemo(() => {
    const map = new Map<string, PermissionEntry[]>();
    for (const item of filtered) {
      const cat = item.category || "通用管理";
      const list = map.get(cat) ?? [];
      list.push(item);
      map.set(cat, list);
    }
    return Array.from(map.entries());
  }, [filtered]);

  const selectedSet = useMemo(() => new Set(value), [value]);

  function toggleOne(id: string, next: boolean) {
    if (next) onChange(selectedSet.has(id) ? [...value] : [...value, id]);
    else onChange(value.filter(item => item !== id));
  }

  function toggleCategory(items: readonly PermissionEntry[]) {
    const allChecked = items.every(item => selectedSet.has(item.id));
    if (allChecked) {
      const drop = new Set(items.map(item => item.id));
      onChange(value.filter(id => !drop.has(id)));
    } else {
      const next = [...value];
      for (const item of items) {
        if (!selectedSet.has(item.id)) next.push(item.id);
      }
      onChange(next);
    }
  }

  return (
    <FieldBoundary>
      <div className="weir-perm-picker">
      <InlineStack justify="between" wrap>
        <SearchInput value={keyword} onChange={setKeyword} placeholder="搜索权限名称或标识" />
        <Badge variant="soft" tone={value.length > 0 ? "info" : "neutral"}>已选 {value.length} 项</Badge>
      </InlineStack>
      {categories.length === 0 ? (
        <div className="muted text-fine">没有匹配的权限项</div>
      ) : (
        <div className="weir-perm-groups">
          {categories.map(([cat, items]) => {
            const allChecked = items.every(item => selectedSet.has(item.id));
            return (
              <div key={cat} className="weir-perm-cat">
                <div className="weir-perm-cat-head">
                  <span>{cat}</span>
                  <Button type="button" size="xs" variant="ghost" onClick={() => toggleCategory(items)}>
                    {allChecked ? "取消本组" : "全选本组"}
                  </Button>
                </div>
                <div className="weir-perm-grid">
                  {items.map(item => (
                    <Checkbox
                      key={item.id}
                      label={item.label}
                      checked={selectedSet.has(item.id)}
                      onChange={checked => toggleOne(item.id, checked)}
                      inline
                    >
                      <span title={item.id}>{item.label}</span>
                    </Checkbox>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}
      </div>
    </FieldBoundary>
  );
}

export function AccessRuleBuilder({
  permissions,
  value,
  onChange,
}: {
  permissions: readonly PermissionEntry[];
  value: readonly ScopedAccessRule[];
  onChange: (next: ScopedAccessRule[]) => void;
}) {
  const update = (index: number, patch: Partial<ScopedAccessRule>) =>
    onChange(value.map((rule, i) => (i === index ? {...rule, ...patch} : rule)));

  return (
    <FieldBoundary>
      <Stack>
        {value.map((rule, index) => (
        <div key={index} className="weir-rule-card">
          <div className="weir-rule-card-head">
            <strong>范围规则 #{index + 1}</strong>
            <Button
              type="button"
              size="icon-xs"
              variant="ghost"
              title="删除规则"
              aria-label={`删除范围规则 ${index + 1}`}
              onClick={() => onChange(value.filter((_, i) => i !== index))}
            >
              <Trash2 aria-hidden />
            </Button>
          </div>
          <Field label="授予的资源权限">
            <PermissionPicker
              permissions={permissions}
              scopeFilter="scoped"
              value={rule.permissions}
              onChange={perms => update(index, {permissions: perms})}
            />
          </Field>
          <InsetRow label="适用全部分组">
            <Switch
              label={`规则 ${index + 1} 适用全部分组`}
              checked={rule.all_groups}
              onChange={all_groups => update(index, {all_groups})}
            />
          </InsetRow>
          {!rule.all_groups && (
            <>
              <Field label="限定调度分组">
                <MultiGroupSelect
                  value={rule.group_ids}
                  onChange={group_ids => update(index, {group_ids})}
                />
              </Field>
              <InsetRow label="包含未分组资源">
                <Switch
                  label={`规则 ${index + 1} 包含未分组资源`}
                  checked={rule.include_ungrouped}
                  onChange={include_ungrouped => update(index, {include_ungrouped})}
                />
              </InsetRow>
            </>
          )}
          <Field label="限定平台（不选代表不限平台）">
            <div className="weir-perm-grid">
              {PLATFORM_OPTIONS.filter(opt => opt.value !== "").map(opt => (
                <Checkbox
                  key={opt.value}
                  label={opt.label}
                  checked={rule.platforms.includes(opt.value)}
                  onChange={checked =>
                    update(index, {
                      platforms: checked ? [...rule.platforms, opt.value] : rule.platforms.filter(p => p !== opt.value),
                    })
                  }
                  inline
                >
                  {opt.label}
                </Checkbox>
              ))}
            </div>
          </Field>
          <Field label="限定账号接入类型（不选代表不限类型）">
            <div className="weir-perm-grid">
              {accountTypeOptions.map(([id, title]) => (
                <Checkbox
                  key={id}
                  label={title}
                  checked={rule.account_types.includes(id)}
                  onChange={checked =>
                    update(index, {
                      account_types: checked
                        ? [...rule.account_types, id]
                        : rule.account_types.filter(t => t !== id),
                    })
                  }
                  inline
                >
                  {title}
                </Checkbox>
              ))}
            </div>
          </Field>
        </div>
      ))}
      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={() =>
          onChange([
            ...value,
            {
              permissions: [],
              group_ids: [],
              all_groups: false,
              include_ungrouped: false,
              account_types: [],
              platforms: [],
            },
          ])
        }
      >
        <Plus aria-hidden />添加范围规则
      </Button>
      </Stack>
    </FieldBoundary>
  );
}

export type AudienceCondition =
  | {type: "subscription"; operator: "in"; group_ids: number[]}
  | {type: "balance"; operator: "gt" | "gte" | "lt" | "lte" | "eq"; value: number};

const balanceComparisons: Record<Extract<AudienceCondition, {type: "balance"}>["operator"], string> = {
  gt: "大于",
  gte: "大于等于",
  lt: "小于",
  lte: "小于等于",
  eq: "等于",
};

const defaultAudienceCondition: AudienceCondition = {type: "balance", operator: "lt", value: 0};

export function AudienceRuleBuilder({
  value,
  onChange,
}: {
  value: readonly (readonly AudienceCondition[])[];
  onChange: (next: AudienceCondition[][]) => void;
}) {
  const updateGroup = (groupIndex: number, nextGroup: AudienceCondition[]) =>
    onChange(value.map((group, idx) => (idx === groupIndex ? [...nextGroup] : [...group])));

  return (
    <FieldBoundary>
      <Stack>
        {value.map((group, groupIndex) => (
        <div key={groupIndex} className="weir-rule-card">
          <div className="weir-rule-card-head">
            <strong>条件组 #{groupIndex + 1}（组内同时满足）</strong>
            <Button
              type="button"
              size="icon-xs"
              variant="ghost"
              title="删除条件组"
              aria-label={`删除条件组 ${groupIndex + 1}`}
              onClick={() => onChange(value.filter((_, idx) => idx !== groupIndex).map(g => [...g]))}
            >
              <Trash2 aria-hidden />
            </Button>
          </div>
          <Stack>
            {group.map((cond, condIndex) => (
              <div key={condIndex} className="setting-row-card">
                <div className="flex-1 min-w-0">
                  <FormGrid>
                    <Field label="匹配维度">
                      <Select
                        value={cond.type}
                        onChange={e => {
                          const next: AudienceCondition =
                            e.currentTarget.value === "subscription"
                              ? {type: "subscription", operator: "in", group_ids: []}
                              : defaultAudienceCondition;
                          updateGroup(groupIndex, group.map((c, i) => (i === condIndex ? next : c)));
                        }}
                      >
                        <option value="balance">账户余额</option>
                        <option value="subscription">有效订阅</option>
                      </Select>
                    </Field>
                    {cond.type === "subscription" ? (
                      <Field label="目标订阅组">
                        <MultiGroupSelect
                          value={cond.group_ids}
                          subscriptionOnly
                          onChange={group_ids =>
                            updateGroup(
                              groupIndex,
                              group.map((c, i) => (i === condIndex ? {...cond, group_ids} : c)),
                            )
                          }
                        />
                      </Field>
                    ) : (
                      <>
                        <Field label="比较关系">
                          <Select
                            value={cond.operator}
                            onChange={e => {
                              const op = e.currentTarget.value as Extract<AudienceCondition, {type: "balance"}>["operator"];
                              updateGroup(
                                groupIndex,
                                group.map((c, i) => (i === condIndex ? {...cond, operator: op} : c)),
                              );
                            }}
                          >
                            {Object.entries(balanceComparisons).map(([op, label]) => (
                              <option key={op} value={op}>{label}</option>
                            ))}
                          </Select>
                        </Field>
                        <Field label="阈值">
                          <Input
                            type="number"
                            step="any"
                            suffix="USD"
                            required
                            value={Number.isNaN(cond.value) ? "" : cond.value}
                            onChange={e =>
                              updateGroup(
                                groupIndex,
                                group.map((c, i) => (i === condIndex ? {...cond, value: e.currentTarget.valueAsNumber} : c)),
                              )
                            }
                          />
                        </Field>
                      </>
                    )}
                  </FormGrid>
                </div>
                {group.length > 1 && (
                  <Button
                    type="button"
                    size="icon-xs"
                    variant="ghost"
                    title="移除此条件"
                    aria-label={`移除条件 ${condIndex + 1}`}
                    onClick={() => updateGroup(groupIndex, group.filter((_, i) => i !== condIndex))}
                  >
                    <Trash2 aria-hidden />
                  </Button>
                )}
              </div>
            ))}
            <InlineStack>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={group.length >= 50}
                onClick={() => updateGroup(groupIndex, [...group, defaultAudienceCondition])}
              >
                <Plus aria-hidden />添加组内条件
              </Button>
            </InlineStack>
          </Stack>
        </div>
      ))}
      <InlineStack>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={value.length >= 50}
          onClick={() => onChange([...value.map(g => [...g]), [defaultAudienceCondition]])}
        >
          <Plus aria-hidden />添加“或”条件组
        </Button>
      </InlineStack>
      </Stack>
    </FieldBoundary>
  );
}
