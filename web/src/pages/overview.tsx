import {useState} from "react";
import {ArrowUpRight} from "lucide-react";
import {z} from "zod";
import {api, empty, subscriptions, type Config} from "../api";
import {money} from "../format";
import {useAction, useResource} from "../requests";
import {Button, DataTable, ErrorLine, Feedback, Field, Form, Input, Resource, Section, Toolbar} from "../ui";
import {subscriptionColumns} from "./records";

const balanceSchema = z.object({balance: z.number().nullable()});

export function Overview({config}: {config: Config}) {
  const [revision, revise] = useState(0);
  const [code, setCode] = useState("");
  const [redeemed, setRedeemed] = useState(false);
  const balance = useResource(() => api("/balance", balanceSchema), "balance", true, revision);
  const plans = useResource(() => api("/subscriptions", z.array(subscriptions)), "subscriptions");
  const action = useAction();
  return <>
    <Section className="balance-strip">
      <div className="balance-value">
        <h2>可用余额</h2>
        <Resource state={balance}>{row => <strong>{money(row.balance)}</strong>}</Resource>
      </div>
      <Form busy={action.busy} onSubmit={() => void action.run(async () => {
        setRedeemed(false);
        await api("/redeem", empty, "POST", {code});
        setCode("");
        setRedeemed(true);
        revise(previous => previous + 1);
      })}>
        <Toolbar>
          <Field label="余额兑换码"><Input required maxLength={128} value={code} onChange={event => {
            setCode(event.target.value);
            setRedeemed(false);
          }} /></Field>
          <Button type="submit" busy={action.busy}>兑换</Button>
        </Toolbar>
        <ErrorLine value={action.error} />
        {redeemed && <Feedback tone="success">兑换成功</Feedback>}
      </Form>
    </Section>
    <Section>
      <Toolbar className="section-heading"><h2>我的订阅</h2>
        <Button variant="outline" render={<a href={config.official + "/purchase"} />}>前往官方购买 <ArrowUpRight aria-hidden /></Button>
      </Toolbar>
      <Resource state={plans}>{rows => <DataTable label="我的订阅" rows={rows} rowID={row => row.id} columns={subscriptionColumns} />}</Resource>
    </Section>
  </>;
}
