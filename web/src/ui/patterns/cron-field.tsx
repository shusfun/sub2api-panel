import {useState} from "react";
import {Input} from "../primitives/input";
import {Button} from "../primitives/button";

export type CronPreset = {
  label: string;
  value: string;
};

const DEFAULT_PRESETS: readonly CronPreset[] = [
  {label: "每小时", value: "0 * * * *"},
  {label: "每 6 小时", value: "0 */6 * * *"},
  {label: "每日 03:00", value: "0 3 * * *"},
  {label: "每周日 04:00", value: "0 4 * * 0"},
];

export function CronField({
  value,
  onChange,
  presets = DEFAULT_PRESETS,
  placeholder = "0 3 * * *",
  disabled = false,
  name,
}: {
  value: string;
  onChange: (next: string) => void;
  presets?: readonly CronPreset[];
  placeholder?: string;
  disabled?: boolean;
  name?: string;
}) {
  const isPreset = presets.some(p => p.value === value);
  const [customMode, setCustomMode] = useState(() => Boolean(value) && !isPreset);

  return (
    <div className="weir-cron-field">
      <div className="weir-cron-presets" role="group" aria-label="常用执行周期">
        {presets.map(preset => {
          const active = !customMode && value === preset.value;
          return (
            <Button
              key={preset.value}
              type="button"
              size="xs"
              variant={active ? "secondary" : "outline"}
              disabled={disabled}
              aria-pressed={active}
              onClick={() => {
                setCustomMode(false);
                onChange(preset.value);
              }}
            >
              {preset.label}
            </Button>
          );
        })}
        <Button
          type="button"
          size="xs"
          variant={customMode ? "secondary" : "ghost"}
          disabled={disabled}
          aria-pressed={customMode}
          onClick={() => setCustomMode(v => !v)}
        >
          自定义 Cron
        </Button>
      </div>
      {(customMode || !isPreset) && (
        <Input
          name={name}
          value={value}
          disabled={disabled}
          placeholder={placeholder}
          className="code-mono"
          onChange={e => onChange(e.target.value)}
        />
      )}
    </div>
  );
}
