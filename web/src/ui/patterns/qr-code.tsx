import {useEffect, useState} from "react";
import {Button} from "../primitives/button";
import {Skeleton} from "./feedback";

export function QrCode({
  value,
  alt,
  expired = false,
  onRefresh,
}: {
  value: string;
  alt: string;
  expired?: boolean;
  onRefresh?: () => void;
}) {
  const [dataURL, setDataURL] = useState("");

  useEffect(() => {
    if (!value) {
      setDataURL("");
      return;
    }
    let cancelled = false;
    void import("qrcode-generator").then(({default: qrcode}) => {
      if (cancelled) return;
      const qr = qrcode(0, "M");
      qr.addData(value);
      qr.make();
      setDataURL(qr.createDataURL(5, 1));
    });
    return () => { cancelled = true; };
  }, [value]);

  return (
    <div className="weir-qr-box">
      {dataURL ? <img src={dataURL} alt={alt} /> : <Skeleton className="w-full h-full" />}
      {expired && (
        <div className="weir-qr-expired">
          <span>二维码已过期</span>
          {onRefresh && (
            <Button type="button" size="xs" variant="outline" onClick={onRefresh}>
              重新生成
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
