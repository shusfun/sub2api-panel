import {Badge} from "./badge";
import {Button} from "./button";
import {CopyButton} from "./copy-button";
import {CopyValue} from "./copy-value";
import {Feedback} from "./feedback";
import {Stack} from "./layout";
import {Overlay} from "./overlay";

export function MaskedSecret({
  masked,
  label = "复制密钥",
  getSecret,
}: {
  masked: string;
  label?: string;
  getSecret?: () => string | Promise<string>;
}) {
  if (!masked) return <span className="muted">—</span>;
  return (
    <span className="weir-masked-secret">
      <Badge variant="tag" className="code-mono">{masked}</Badge>
      {getSecret && <CopyButton getText={getSecret} label={label} size="icon-xs" />}
    </span>
  );
}

export function SecretRevealDialog({
  open,
  title,
  label,
  value,
  values,
  notice = "明文凭据仅在本次创建或轮换成功时显示一次，关闭后无法再次查看。",
  onClose,
}: {
  open: boolean;
  title: string;
  label: string;
  value?: string;
  values?: readonly string[];
  notice?: string;
  onClose: () => void;
}) {
  const list = values ?? (value ? [value] : []);
  return (
    <Overlay
      open={open && list.length > 0}
      title={title}
      onClose={onClose}
      footer={
        <>
          {list.length > 1 && <CopyButton value={list.join("\n")} label="复制全部" variant="outline">复制全部</CopyButton>}
          <Button onClick={onClose}>我已妥善保存</Button>
        </>
      }
    >
      <Feedback>{notice}</Feedback>
      {list.length === 1 ? (
        <CopyValue label={label} value={list[0]} />
      ) : (
        <Stack className="code-issue-list">
          {list.map((code, idx) => (
            <CopyValue key={`${idx}-${code}`} label={`${label} #${idx + 1}`} value={code} />
          ))}
        </Stack>
      )}
    </Overlay>
  );
}
