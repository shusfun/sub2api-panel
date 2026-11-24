import {useState} from "react";
import {FilterBar, Segmented, WorkspaceContent, WorkspaceStage} from "@/ui";
import {ChannelMonitor} from "./channel-monitor";
import {PatrolPolicyDialog, ProbeTemplatesDialog} from "./patrol-admin";
import {PublicPatrolSection} from "./patrol-board";

export {PublicPatrolSection};

export function PatrolWorkspace({admin, monitorEnabled = false}: {admin: boolean; monitorEnabled?: boolean}) {
  const [view, setView] = useState<"patrol" | "monitor">("patrol");
  const [revision, revise] = useState(0);
  const [editingGroup, setEditingGroup] = useState<number | "new">();
  const [templatesOpen, setTemplatesOpen] = useState(false);

  return (
    <WorkspaceStage flush={monitorEnabled}>
      {monitorEnabled && (
        <FilterBar sticky>
          <div className="obs-filter-row">
            <Segmented
              label="渠道健康视角"
              value={view}
              options={[
                ["patrol", "智能巡检"],
                ["monitor", "官方渠道监控"],
              ] as const}
              onChange={setView}
            />
          </div>
        </FilterBar>
      )}

      {monitorEnabled && view === "monitor" ? (
        <ChannelMonitor admin={admin} />
      ) : (
        <WorkspaceContent>
          <PublicPatrolSection
            publicMode={false}
            admin={admin}
            revision={revision}
            onEditPolicy={admin ? gid => setEditingGroup(gid ?? "new") : undefined}
            onOpenTemplates={admin ? () => setTemplatesOpen(true) : undefined}
          />
        </WorkspaceContent>
      )}

      {admin && (
        <>
          <PatrolPolicyDialog
            target={editingGroup}
            revision={revision}
            onClose={() => setEditingGroup(undefined)}
            onSaved={() => {
              setEditingGroup(undefined);
              revise(v => v + 1);
            }}
          />
          <ProbeTemplatesDialog
            open={templatesOpen}
            onClose={() => setTemplatesOpen(false)}
          />
        </>
      )}
    </WorkspaceStage>
  );
}
