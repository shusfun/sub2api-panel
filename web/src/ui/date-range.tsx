import {Form} from "./form";
import {Button} from "./button";
import {Field} from "./field";
import {Input} from "./input";

export function DateRange({
  start,
  end,
  onChange,
  onApply,
  applyLabel = "应用区间",
}: {
  start: string;
  end: string;
  onChange: (range: {start: string; end: string}) => void;
  onApply: () => void;
  applyLabel?: string;
}) {
  return (
    <Form className="obs-date-range" onSubmit={onApply}>
      <Field label="开始日期">
        <Input type="date" value={start} required max={end || undefined} onChange={event => onChange({start: event.target.value, end})} />
      </Field>
      <Field label="结束日期">
        <Input type="date" value={end} required min={start || undefined} onChange={event => onChange({start, end: event.target.value})} />
      </Field>
      <Button type="submit" disabled={!start || !end || end < start}>{applyLabel}</Button>
    </Form>
  );
}
