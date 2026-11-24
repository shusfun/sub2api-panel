import {useEffect, useState} from "react";
import {Button} from "./ui/button";
import {ThemeIcon} from "./ui/theme-icon";

type Theme = "light" | "dark" | "system";

export const themeNames: Record<Theme, string> = {light: "浅色", dark: "深色", system: "跟随系统"};
const readTheme = (): Theme => {
  const value = localStorage.getItem("panel-theme");
  return value === "light" || value === "dark" ? value : "system";
};
const themeEvent = "panel-theme-change";

// 主题在顶栏按钮和设置页之间共享：写入本地存储并广播，各处订阅同一个值。
export function useTheme(): [Theme, (next: Theme) => void] {
  const [theme, setLocal] = useState<Theme>(readTheme);
  useEffect(() => {
    const sync = () => setLocal(readTheme());
    window.addEventListener(themeEvent, sync);
    return () => window.removeEventListener(themeEvent, sync);
  }, []);
  return [theme, next => { localStorage.setItem("panel-theme", next); window.dispatchEvent(new Event(themeEvent)); }];
}

export function ThemeControl() {
  const [theme, setTheme] = useTheme();
  useEffect(() => {
    const system = matchMedia("(prefers-color-scheme: dark)");
    const update = () => {
      document.documentElement.classList.add("weir-theme-switching");
      document.documentElement.classList.toggle("dark", theme === "dark" || theme === "system" && system.matches);
      requestAnimationFrame(() => document.documentElement.classList.remove("weir-theme-switching"));
    };
    update();
    system.addEventListener("change", update);
    return () => system.removeEventListener("change", update);
  }, [theme]);
  const label = "主题 · " + themeNames[theme];
  return (
    <Button variant="ghost" size="icon" title={label} aria-label={label}
      onClick={() => setTheme(theme === "system" ? "light" : theme === "light" ? "dark" : "system")}>
      <ThemeIcon mode={theme} />
    </Button>
  );
}

export function Brand({name = "Sub2API", logo}: {name?: string; logo?: string}) {
  return (
    <div className="brand">
      <div className="brand-icon">
        <img src={logo || "/favicon.svg"} alt="" />
      </div>
      <span className="brand-name" title={name}>{name}</span>
    </div>
  );
}
