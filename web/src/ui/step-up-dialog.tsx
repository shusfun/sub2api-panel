import {useId, useState, type ReactNode} from "react";
import {z} from "zod";
import {api} from "../api";
import {useResource} from "../requests";
import {navigateTo} from "../route";
import {Button} from "./button";
import {ErrorLine, Feedback, Resource} from "./feedback";
import {Field} from "./field";
import {Form, FormActions, type FormErrors} from "./form";
import {HoldButton} from "./hold-button";
import {InsetGroup, InsetRow} from "./inset-group";
import {OtpInput} from "./otp-input";
import {Overlay} from "./overlay";
import {PasswordInput} from "./password-input";

const totpStatusSchema = z.object({enabled: z.boolean(), feature_enabled: z.boolean()});

export function useTotpStatus(revision = 0, enabled = true) {
  return useResource(() => api("/settings/totp", totpStatusSchema), "/settings/totp", enabled, revision);
}

export function StepUpDialog({
  open = true,
  title = "安全验证",
  description,
  labels,
  confirmLabel = "验证并继续",
  requireTotp = true,
  requirePassword = false,
  critical = false,
  busy,
  error,
  fieldErrors,
  children,
  onClose,
  onConfirm,
}: {
  open?: boolean;
  title?: string;
  description?: ReactNode;
  labels?: readonly string[];
  confirmLabel?: string;
  requireTotp?: boolean;
  requirePassword?: boolean;
  critical?: boolean;
  busy: boolean;
  error: string;
  fieldErrors?: FormErrors;
  children?: ReactNode;
  onClose: () => void;
  onConfirm: (payload: {totp: string; password: string}) => void;
}) {
  const status = useTotpStatus(0, open && requireTotp);
  const formID = useId();
  const [totp, setTotp] = useState("");
  const [password, setPassword] = useState("");

  const totpReady = !requireTotp || status?.value?.enabled === true;
  const canSubmit = (!requireTotp || /^\d{6}$/.test(totp)) && (!requirePassword || password.length > 0);

  return (
    <Overlay
      open={open}
      title={title}
      onClose={onClose}
      busy={busy}
      footer={
        (!requireTotp || status?.value) && (
          <FormActions>
            <Button variant="ghost" disabled={busy} onClick={onClose}>取消</Button>
            {totpReady ? (
              critical ? (
                <HoldButton
                  busy={busy}
                  disabled={!canSubmit}
                  onConfirm={() => onConfirm({totp: totp.trim(), password})}
                >
                  {confirmLabel}
                </HoldButton>
              ) : (
                <Button form={formID} type="submit" busy={busy} disabled={!canSubmit}>
                  {confirmLabel}
                </Button>
              )
            ) : (
              <Button
                onClick={() => {
                  onClose();
                  navigateTo("settings", {section: "password"});
                }}
              >
                前往绑定验证器
              </Button>
            )}
          </FormActions>
        )
      }
    >
      {description && <Feedback>{description}</Feedback>}
      {labels && labels.length > 0 && (
        <InsetGroup title="本次操作">
          {labels.map(label => <InsetRow key={label} label={label} />)}
        </InsetGroup>
      )}
      {requireTotp ? (
        <Resource state={status}>
          {value =>
            value.enabled ? (
              <Form
                id={formID}
                busy={busy}
                errors={fieldErrors}
                onSubmit={() => onConfirm({totp: totp.trim(), password})}
              >
                {children}
                {requirePassword && (
                  <Field label="当前登录密码" name="password">
                    <PasswordInput
                      name="password"
                      autoComplete="current-password"
                      value={password}
                      onChange={e => setPassword(e.currentTarget.value)}
                      required
                    />
                  </Field>
                )}
                <Field label="验证器 6 位验证码" name="totp">
                  <OtpInput
                    name="totp"
                    autoFocus={!requirePassword}
                    value={totp}
                    onChange={e => setTotp(e.currentTarget.value)}
                    required
                  />
                </Field>
                <ErrorLine value={error} />
              </Form>
            ) : (
              <Feedback>此敏感操作需要两步验证动态码，当前账号尚未绑定验证器。</Feedback>
            )
          }
        </Resource>
      ) : (
        <Form
          id={formID}
          busy={busy}
          errors={fieldErrors}
          onSubmit={() => onConfirm({totp: "", password})}
        >
          {children}
          {requirePassword && (
            <Field label="当前登录密码" name="password">
              <PasswordInput
                name="password"
                autoFocus
                autoComplete="current-password"
                value={password}
                onChange={e => setPassword(e.currentTarget.value)}
                required
              />
            </Field>
          )}
          <ErrorLine value={error} />
        </Form>
      )}
    </Overlay>
  );
}
