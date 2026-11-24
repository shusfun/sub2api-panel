import {useCallback, useState} from "react";
import {ChevronLeft} from "lucide-react";
import {z} from "zod";
import {api} from "../../api";
import {useResource} from "../../requests";
import {Button, ConfirmDialog, InsetGroup, InsetRow} from "../../ui";
import {PersonalSection, personalSections} from "./personal";
import {SettingSection} from "./section";
import {SectionTools} from "./tools";

const sectionsSchema = z.object({items: z.array(z.object({id: z.string(), title: z.string(), hint: z.string().optional(), step_up: z.boolean()}))});

// 设置：左侧分区、右侧内容。个人分区所有人可见，站点分区由后端目录给出且仅管理员可见。
export function SettingsWorkspace({admin, section, onSection}: {admin: boolean; section: string; onSection: (id: string) => void}) {
  const site = useResource(() => api("/admin/settings", sectionsSchema), "setting-sections", admin);
  const siteSections = admin ? site?.value?.items ?? [] : [];
  const all = [...personalSections, ...siteSections];
  const active = all.find(item => item.id === section) ?? personalSections[0];
  const personal = personalSections.some(item => item.id === active.id);
  const [dirty, setDirty] = useState(false);
  const [leaving, setLeaving] = useState<string>();
  const markDirty = useCallback((value: boolean) => setDirty(value), []);
  const go = (id: string) => dirty && id !== active.id ? setLeaving(id) : onSection(id);

  return (
    <div className="settings-layout" data-detail={section ? "" : undefined}>
      <nav className="settings-nav" aria-label="设置分区">
        <InsetGroup title="个人">
          {personalSections.map(item => <InsetRow key={item.id} label={item.title} active={item.id === active.id} onClick={() => go(item.id)} />)}
        </InsetGroup>
        {admin && <InsetGroup title="站点" footer={site?.error || undefined}>
          {siteSections.map(item => <InsetRow key={item.id} label={item.title} active={item.id === active.id} onClick={() => go(item.id)} />)}
          {!siteSections.length && <InsetRow label={site?.error ? "读不到站点设置" : "正在加载…"} />}
        </InsetGroup>}
      </nav>
      <section className="settings-body" aria-label={active.title}>
        <Button variant="ghost" className="settings-back" onClick={() => go("")}><ChevronLeft aria-hidden />设置分区</Button>
        <h2 className="settings-title">{active.title}</h2>
        {personal ? <PersonalSection key={active.id} id={active.id} /> : <>
          {active.id !== "access-keys" && <SettingSection key={active.id} id={active.id} onDirty={markDirty} />}
          <SectionTools id={active.id} />
        </>}
      </section>
      <ConfirmDialog open={leaving !== undefined} title="放弃未保存的改动" description="这个分区有改动还没保存，离开后会丢失。" confirm="放弃改动" busy={false} error=""
        onClose={() => setLeaving(undefined)} onConfirm={() => { const next = leaving ?? ""; setLeaving(undefined); setDirty(false); onSection(next); }} />
    </div>
  );
}
