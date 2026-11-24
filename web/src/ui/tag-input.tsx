import {useState} from "react";
import {X} from "lucide-react";
import {Button} from "./button";
import {FieldBoundary, useFieldControl} from "./field";

export function TagInput({
  name,
  value,
  placeholder = "输入后按回车添加",
  disabled,
  validate,
  onChange,
}: {
  name?: string;
  value: readonly string[];
  placeholder?: string;
  disabled?: boolean;
  validate?: (item: string) => string | undefined;
  onChange: (next: string[]) => void;
}) {
  const {attributes, clearError} = useFieldControl({name});
  const [draft, setDraft] = useState("");
  const [localError, setLocalError] = useState("");

  function commitRaw(raw: string) {
    const tokens = raw.split(/[\n,，\s]+/).map(item => item.trim()).filter(Boolean);
    if (tokens.length === 0) return true;
    const next = [...value];
    for (const token of tokens) {
      if (validate) {
        const reason = validate(token);
        if (reason) {
          setLocalError(reason);
          return false;
        }
      }
      if (!next.includes(token)) next.push(token);
    }
    setLocalError("");
    clearError?.();
    onChange(next);
    return true;
  }

  return (
    <FieldBoundary>
      <div className="stack">
      <div className="weir-tag-box">
        {value.map(tag => (
          <span key={tag} className="weir-tag-pill">
            <span className="code-mono">{tag}</span>
            <Button
              type="button"
              size="icon-xs"
              variant="ghost"
              title={`移除 ${tag}`}
              aria-label={`移除 ${tag}`}
              disabled={disabled}
              onClick={() => onChange(value.filter(item => item !== tag))}
            >
              <X aria-hidden />
            </Button>
          </span>
        ))}
        <input
          {...attributes}
          value={draft}
          disabled={disabled}
          placeholder={value.length === 0 ? placeholder : ""}
          aria-invalid={localError ? true : attributes?.["aria-invalid"]}
          className="weir-tag-entry"
          onChange={e => {
            setDraft(e.currentTarget.value);
            if (localError) setLocalError("");
          }}
          onKeyDown={e => {
            if (e.key === "Enter" || e.key === ",") {
              if (!draft.trim()) return;
              e.preventDefault();
              if (commitRaw(draft)) setDraft("");
            } else if (e.key === "Backspace" && !draft && value.length > 0) {
              onChange(value.slice(0, -1));
            }
          }}
          onPaste={e => {
            const text = e.clipboardData.getData("text");
            if (/[\n,，\s]/.test(text)) {
              e.preventDefault();
              commitRaw(text);
            }
          }}
          onBlur={() => {
            if (draft.trim() && commitRaw(draft)) setDraft("");
          }}
        />
      </div>
        {localError && <div className="field-error" role="alert">{localError}</div>}
      </div>
    </FieldBoundary>
  );
}
