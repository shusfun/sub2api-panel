import {createContext, useContext, useId, useLayoutEffect, useState, type AriaAttributes, type ComponentProps, type ReactNode} from "react";
import {cn} from "cn";
import {useFormContext, type FormErrors} from "./form";

type Identity = {id?: string; name?: string};
type ControlProps = Identity & Pick<AriaAttributes, "aria-describedby" | "aria-labelledby" | "aria-invalid" | "aria-label">;
type ValidationControl = HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;
const FieldContext = createContext<{
  id: string;
  name?: string;
  labelID: string;
  description: string;
  invalid: boolean;
  bind: (control: Identity) => void;
  clearError: () => void;
} | null>(null);

export function FieldBoundary({children}: {children: ReactNode}) {
  return <FieldContext.Provider value={null}>{children}</FieldContext.Provider>;
}

export function useFieldControl(props: ControlProps = {}) {
  const field = useContext(FieldContext);
  const id = props.id ?? field?.id;
  const name = props.name ?? field?.name;
  const bind = field?.bind;
  useLayoutEffect(() => {
    if (bind && id) bind({id, name});
  }, [bind, id, name]);
  return {
    attributes: field ? {
      id, name,
      "aria-labelledby": props["aria-labelledby"] ?? (props["aria-label"] ? undefined : field.labelID),
      "aria-describedby": [props["aria-describedby"], field.description].filter(Boolean).join(" ") || undefined,
      "aria-invalid": field.invalid ? true : props["aria-invalid"],
    } : undefined,
    clearError: field?.clearError,
  };
}

function validationMessage(control: ValidationControl) {
  const {validity} = control;
  if (validity.customError) return control.validationMessage;
  if (validity.badInput) return "请输入有效的值。";
  if (validity.valueMissing) {
    if (control instanceof HTMLSelectElement) return "请选择一项。";
    if (control instanceof HTMLInputElement && control.type === "checkbox") return "请勾选此项。";
    return "请填写此字段。";
  }
  if (validity.typeMismatch && control instanceof HTMLInputElement) {
    return control.type === "email" ? "请输入有效的邮箱地址。" : "请输入有效的网址。";
  }
  if (validity.patternMismatch) return control.title || "请输入符合要求的格式。";
  if (validity.tooShort && "minLength" in control) return "请输入至少 " + control.minLength + " 个字符。";
  if (validity.tooLong && "maxLength" in control) return "输入不能超过 " + control.maxLength + " 个字符。";
  if (validity.rangeUnderflow && control instanceof HTMLInputElement) return "值不能小于 " + control.min + "。";
  if (validity.rangeOverflow && control instanceof HTMLInputElement) return "值不能大于 " + control.max + "。";
  if (validity.stepMismatch) return "请输入符合步长要求的值。";
  return "请检查此字段。";
}

export function Field({label, hint, error, name, id, className, children, ...props}: Omit<ComponentProps<"div">, "id" | "onInvalid"> & {
  label: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  name?: string;
  id?: string;
}) {
  const baseID = useId();
  const controlID = id ?? baseID;
  const labelID = baseID + "-label";
  const hintID = baseID + "-hint";
  const errorID = baseID + "-error";
  const [control, setControl] = useState<Identity>();
  const bindControl = (next: Identity) => {
    setControl(prev => (prev?.id === next.id && prev?.name === next.name ? prev : next));
  };
  const [nativeError, setNativeError] = useState("");
  const [dismissedErrors, dismissErrors] = useState<FormErrors>();
  const form = useFormContext();
  const fieldName = control?.name ?? name;
  const errors = form?.errors;
  const serverError = fieldName && errors && errors !== dismissedErrors && Object.hasOwn(errors, fieldName)
    ? errors[fieldName] : undefined;
  const message = error || serverError || nativeError;
  const primaryID = control?.id ?? controlID;
  useLayoutEffect(() => {
    if (!nativeError) return;
    const target = document.getElementById(primaryID);
    if ((target instanceof HTMLInputElement || target instanceof HTMLSelectElement || target instanceof HTMLTextAreaElement) && target.validity.valid) {
      setNativeError("");
    }
  });
  function clearError() {
    setNativeError("");
    if (serverError) dismissErrors(errors);
  }
  return <div {...props} className={cn("field", className)}
    onInput={event => {
      props.onInput?.(event);
      if (event.target instanceof HTMLElement && event.target.id === primaryID) clearError();
    }}
    onInvalid={event => {
      const target = event.target;
      if (!(target instanceof HTMLInputElement || target instanceof HTMLSelectElement || target instanceof HTMLTextAreaElement) ||
        target.id !== primaryID || !event.currentTarget.contains(target)) return;
      event.preventDefault();
      setNativeError(validationMessage(target));
    }}>
    {typeof label === "string" ? <label id={labelID} className="field-label" htmlFor={primaryID}>{label}</label> : <span id={labelID} className="field-label">{label}</span>}
    <FieldContext.Provider value={{id: controlID, name, labelID, invalid: !!message, description: [hint && hintID, message && errorID].filter(Boolean).join(" "), bind: bindControl, clearError}}>
      {children}
    </FieldContext.Provider>
    {hint && <div id={hintID} className="field-hint">{hint}</div>}
    {message && <div id={errorID} className="field-error">{message}</div>}
  </div>;
}
