import {useEffect, useState} from "react";
import {CircleDollarSign} from "lucide-react";
import {Button} from "./ui/button";
import {ThemeIcon} from "./ui/theme-icon";

type Theme = "light" | "dark" | "system";

const themeNames: Record<Theme, string> = {light: "浅色", dark: "深色", system: "跟随系统"};
export function ThemeControl() {
  const [theme, setTheme] = useState<Theme>(() => {
    const value = localStorage.getItem("panel-theme");
    return value === "light" || value === "dark" ? value : "system";
  });
  useEffect(() => {
    const system = matchMedia("(prefers-color-scheme: dark)");
    const update = () => document.documentElement.classList.toggle("dark", theme === "dark" || theme === "system" && system.matches);
    update();
    system.addEventListener("change", update);
    localStorage.setItem("panel-theme", theme);
    return () => system.removeEventListener("change", update);
  }, [theme]);
  const label = "主题 · " + themeNames[theme];
  return <Button variant="ghost" size="icon" title={label} aria-label={label}
    onClick={() => setTheme(theme === "system" ? "light" : theme === "light" ? "dark" : "system")}><ThemeIcon mode={theme} /></Button>;
}

export function Brand() {
  return <div className="brand"><div className="brand-icon"><CircleDollarSign aria-hidden /></div><span>Sub2API</span></div>;
}
