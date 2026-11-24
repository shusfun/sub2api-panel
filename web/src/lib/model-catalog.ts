import {z} from "zod";
import {api} from "./http";
import {useResource} from "./requests";
import {modelOptions, type ModelCatalog} from "@/ui/domain/model-select";

const modelIDs = z.object({models: z.array(z.string())});
const accountModels = z.array(z.object({id: z.string(), display_name: z.string()}));

export function useModelCatalog(path: string, revision: number, enabled = true, withDisplayNames = false): ModelCatalog {
  const state = useResource(async () => ({
    path,
    options: withDisplayNames
      ? (await api(path, accountModels)).map(({id, display_name}) => ({value: id, label: display_name || id, meta: display_name && display_name !== id ? id : undefined}))
      : modelOptions((await api(path, modelIDs)).models),
  }), path, enabled, revision);
  return {options: state?.value?.path === path ? state.value.options : undefined, loading: state?.loading, error: state?.error};
}
