import {useEffect, useId, useRef, useState} from "react";
import {Upload, X} from "lucide-react";
import {Button} from "./button";
import {FieldBoundary, useFieldControl} from "./field";
import {EllipsisText} from "./tooltip";

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / (1024 * 1024)).toFixed(2)} MiB`;
}

function matchesAccept(file: File, accept?: string): boolean {
  if (!accept) return true;
  const rules = accept.split(",").map(item => item.trim().toLowerCase()).filter(Boolean);
  if (rules.length === 0) return true;
  const name = file.name.toLowerCase();
  const mime = file.type.toLowerCase();
  return rules.some(rule => {
    if (rule.startsWith(".")) return name.endsWith(rule);
    if (rule.endsWith("/*")) return mime.startsWith(rule.slice(0, -1));
    return mime === rule;
  });
}

export function FilePicker({
  name,
  accept,
  multiple = false,
  maxBytes,
  maxFiles,
  hint,
  value,
  disabled,
  required,
  onChange,
}: {
  name?: string;
  accept?: string;
  multiple?: boolean;
  maxBytes?: number;
  maxFiles?: number;
  hint?: string;
  value: readonly File[];
  disabled?: boolean;
  required?: boolean;
  onChange: (files: File[]) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const fallbackID = useId();
  const {attributes, clearError} = useFieldControl({name});
  const [dragging, setDragging] = useState(false);
  const [localError, setLocalError] = useState("");
  const [previews, setPreviews] = useState<Record<string, string>>({});

  useEffect(() => {
    const next: Record<string, string> = {};
    for (const file of value) {
      if (file.type.startsWith("image/")) {
        next[file.name + ":" + file.size] = URL.createObjectURL(file);
      }
    }
    setPreviews(next);
    return () => {
      for (const url of Object.values(next)) URL.revokeObjectURL(url);
    };
  }, [value]);

  function applyFiles(incoming: FileList | null) {
    if (!incoming || incoming.length === 0) return;
    const picked = Array.from(incoming);
    const merged = multiple ? [...value, ...picked] : picked.slice(0, 1);
    if (maxFiles && merged.length > maxFiles) {
      setLocalError(`最多可选择 ${maxFiles} 个文件。`);
      return;
    }
    for (const file of merged) {
      if (!matchesAccept(file, accept)) {
        setLocalError(`文件「${file.name}」格式不符合要求。`);
        return;
      }
      if (maxBytes && file.size > maxBytes) {
        setLocalError(`文件「${file.name}」超过上限 ${formatFileSize(maxBytes)}。`);
        return;
      }
    }
    setLocalError("");
    clearError?.();
    onChange(merged);
  }

  const inputID = attributes?.id ?? fallbackID;

  return (
    <FieldBoundary>
      <div className="weir-file-picker">
      <label
        htmlFor={inputID}
        data-dragging={dragging || undefined}
        className="weir-file-drop"
        onDragOver={e => {
          e.preventDefault();
          if (!disabled) setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={e => {
          e.preventDefault();
          setDragging(false);
          if (!disabled) applyFiles(e.dataTransfer.files);
        }}
      >
        <input
          ref={inputRef}
          id={inputID}
          name={attributes?.name ?? name}
          type="file"
          accept={accept}
          multiple={multiple}
          disabled={disabled}
          required={required && value.length === 0}
          aria-describedby={attributes?.["aria-describedby"]}
          aria-invalid={localError ? true : attributes?.["aria-invalid"]}
          onChange={e => {
            applyFiles(e.currentTarget.files);
            e.currentTarget.value = "";
          }}
        />
        <div className="weir-file-meta">
          <strong>{multiple ? "点击选择或拖拽多个文件到此处" : "点击选择或拖拽文件到此处"}</strong>
          {hint && <span>{hint}</span>}
        </div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={disabled}
          onClick={e => {
            e.preventDefault();
            inputRef.current?.click();
          }}
        >
          <Upload aria-hidden />选择文件
        </Button>
      </label>

      {localError && <div className="field-error" role="alert">{localError}</div>}

      {value.length > 0 && (
        <div className="weir-file-list">
          {value.map((file, index) => {
            const thumb = previews[file.name + ":" + file.size];
            return (
              <div key={`${file.name}-${file.lastModified}-${index}`} className="weir-file-item">
                {thumb && <img src={thumb} alt="" className="weir-file-thumb" />}
                <EllipsisText text={file.name} className="weir-file-name" />
                <span className="weir-file-size">{formatFileSize(file.size)}</span>
                <Button
                  type="button"
                  size="icon-xs"
                  variant="ghost"
                  title="移除文件"
                  aria-label={`移除 ${file.name}`}
                  disabled={disabled}
                  onClick={() => onChange(value.filter((_, i) => i !== index))}
                >
                  <X aria-hidden />
                </Button>
              </div>
            );
          })}
        </div>
      )}
      </div>
    </FieldBoundary>
  );
}
