import {useEffect, useState} from "react";
import {api} from "@/lib/http";
import {empty} from "@/schemas";
import {ledgerSettings, type LedgerSettings} from "@/schemas/ledger";
import {useAction, useResource} from "@/lib/requests";
import {Field, Form, FormSection, Input, InsetRow, Resource, SaveBar, Switch, toast} from "@/ui";

// 契约桩（第 0 步定死签名）：线 D 负责毛利开关、告警阈值与保留天数；线 A 负责接进站点设置。
export const ledgerSettingsSection = {id: "ledger", title: "上游账本"} as const;

type NumberKey = Exclude<keyof LedgerSettings, "show_profit">;

const numberFields: {key: NumberKey; label: string; hint: string; step: string; min: number}[] = [
  {key: "deviation_pct", label: "偏差百分比", hint: "账本余额与上游余额的偏差超过该百分比时告警", step: "any", min: 0},
  {key: "deviation_min_usd", label: "偏差最低金额（USD）", hint: "偏差金额低于该值时不告警", step: "any", min: 0},
  {key: "runway_hours", label: "可撑小时", hint: "按近期消耗估算，余额可撑时长低于该小时数时告警", step: "any", min: 0},
  {key: "failure_threshold", label: "失败阈值", hint: "连续同步失败达到该次数时告警", step: "1", min: 1},
  {key: "hour_retention_days", label: "小时账本保留天数", hint: "超过天数的小时汇总会被清理", step: "1", min: 1},
  {key: "sample_retention_days", label: "采样保留天数", hint: "超过天数的余额采样会被清理", step: "1", min: 1},
];

export function LedgerSettingsSection({onDirty}: {onDirty: (dirty: boolean) => void}) {
  const [revision, revise] = useState(0);
  const settings = useResource(() => api("/admin/ledger/settings", ledgerSettings), "ledger-settings", true, revision);
  const [draft, setDraft] = useState<Partial<LedgerSettings>>({});
  const save = useAction();
  const keys = Object.keys(draft);
  const dirty = keys.length > 0;
  useEffect(() => { onDirty(dirty); }, [dirty, onDirty]);
  useEffect(() => () => onDirty(false), [onDirty]);

  function change<K extends keyof LedgerSettings>(base: LedgerSettings, key: K, value: LedgerSettings[K]) {
    setDraft(previous => {
      const next = {...previous, [key]: value};
      if (base[key] === value) delete next[key];
      return next;
    });
  }

  return (
    <Resource state={settings}>
      {value => {
        const current = {...value, ...draft} as LedgerSettings;
        return (
          <Form
            busy={save.busy}
            errors={save.fieldErrors}
            className="setting-fields"
            noValidate
            onSubmit={() => {
              if (!dirty) return;
              void save.run(async () => {
                await api("/admin/ledger/settings", empty, "PUT", current);
                setDraft({});
                toast("账本设置已保存");
                revise(n => n + 1);
              });
            }}
          >
            <FormSection title="展示">
              <InsetRow label="显示毛利" hint="在账号与账本摘要中显示收入减成本的毛利">
                <Switch label="显示毛利" checked={current.show_profit} onChange={next => change(value, "show_profit", next)} />
              </InsetRow>
            </FormSection>
            <FormSection title="告警阈值与保留">
              {numberFields.map(field => (
                <Field key={field.key} label={field.label} hint={field.hint} name={field.key}>
                  <div className="row-field">
                    <Input
                      aria-label={field.label}
                      type="number"
                      step={field.step}
                      min={field.min}
                      value={String(current[field.key])}
                      onChange={event => change(value, field.key, event.target.value === "" ? 0 : Number(event.target.value))}
                    />
                  </div>
                </Field>
              ))}
            </FormSection>
            <SaveBar
              dirty={dirty}
              dirtyCount={keys.length}
              busy={save.busy}
              error={save.error}
              onReset={() => { setDraft({}); save.clearError(); }}
            />
          </Form>
        );
      }}
    </Resource>
  );
}
