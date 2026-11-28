import {useCallback, useState} from "react";
import {ChevronLeft} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useResource} from "@/lib/requests";
import {Button, ConfirmDialog, InsetGroup, InsetRow} from "@/ui";
import {PersonalSection, personalSections} from "./personal";
import {SettingSection} from "./section";
import {SectionTools} from "./tools";
import {LedgerSettingsSection, ledgerSettingsSection} from "./ledger";

const sectionsSchema = z.object({items: z.array(z.object({id: z.string(), title: z.string(), hint: z.string().optional(), step_up: z.boolean()}))});

// 设置：左侧分区、右侧内容。
// scope=personal（/settings）只渲染个人分区；scope=site（/admin/settings）渲染后端下发的站点分区，再加上面板自有的“上游账本”。
export function SettingsWorkspace({scope, section, onSection}: {scope: "personal" | "site"; section: string; onSection: (id: string) => void}) {
  const site = scope === "site";
  const catalog = useResource(() => api("/admin/settings", sectionsSchema), "setting-sections", site);
  const siteSections = site ? [...(catalog?.value?.items ?? []), ledgerSettingsSection] : [];
  const sections = site ? siteSections : personalSections;
  const active = sections.find(item => item.id === section) ?? sections[0];
  const [dirty, setDirty] = useState(false);
  const [sectionDraft, setSectionDraft] = useState<Record<string, unknown>>({});
  const [leaving, setLeaving] = useState<string>();
  const markDirty = useCallback((value: boolean) => setDirty(value), []);
  const go = (id: string) => dirty && id !== active?.id ? setLeaving(id) : onSection(id);
  const title = site ? "站点设置" : "账户设置";

  return (
    <div className="settings-layout" data-detail={section ? "" : undefined}>
      <nav className="settings-nav" aria-label={title}>
        <InsetGroup title={title} footer={site ? catalog?.error || undefined : undefined}>
          {sections.map(item => <InsetRow key={item.id} label={item.title} active={item.id === active?.id} onClick={() => go(item.id)} />)}
          {site && !catalog?.value && <InsetRow label={catalog?.error ? "读不到站点设置" : "正在加载…"} />}
        </InsetGroup>
      </nav>
      {active && <section className="settings-body" aria-label={active.title}>
        <Button variant="ghost" className="settings-back" onClick={() => go("")}><ChevronLeft aria-hidden />{title}</Button>
        <h2 className="settings-title">{active.title}</h2>
        {!site ? <PersonalSection key={active.id} id={active.id} />
          : active.id === ledgerSettingsSection.id ? <LedgerSettingsSection key={active.id} onDirty={markDirty} />
          : <>
            {active.id !== "access-keys" && <SettingSection key={active.id} id={active.id} onDirty={markDirty} onDraftChange={setSectionDraft} />}
            <SectionTools id={active.id} draft={sectionDraft} />
          </>}
      </section>}
      <ConfirmDialog open={leaving !== undefined} title="放弃未保存的改动" description="这个分区有改动还没保存，离开后会丢失。" confirm="放弃改动" busy={false} error=""
        onClose={() => setLeaving(undefined)} onConfirm={() => { const next = leaving ?? ""; setLeaving(undefined); setDirty(false); onSection(next); }} />
    </div>
  );
}
