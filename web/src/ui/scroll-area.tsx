// 自定义滚动条：底层仍是原生滚动（触控板惯性、键盘翻页、读屏、scrollIntoView 照常），
// overlay 模式不占布局宽度；stable 模式始终预留轨道与焦点环余量，适用于表单弹窗。两者显隐均不改变内容宽度。
// 只在滚动中、或鼠标停在滚动条上时显示；停止滚动后 Base UI 约 500 ms 取消 data-scrolling，
// 再经 CSS 过渡延迟淡出（合计约 600 ms），样式见 styles/controls.css 的 .weir-scroll-*。
//
// 结构：根（className，定尺寸：max-h / h / flex-1 / 边框）→ 视口（viewportClassName，真正的滚动元素，
// 留焦点环余量用 weir-focus-gutter 写在这里）→ 内容（contentClassName，排版：stack / grid / gap / 内边距）。
// 监听 scroll、读写 scrollTop 用 viewportRef；不需要内容层时 contentRender={false}，子元素直接放进视口。
import { type ComponentProps, type CSSProperties, type ReactElement, type ReactNode, type Ref } from "react";
import { ScrollArea as ScrollAreaPrimitive } from "@base-ui/react/scroll-area";
import { cn } from "cn";

type Orientation = "vertical" | "horizontal" | "both";

type ScrollAreaProps = Omit<ComponentProps<"div">, "children" | "content"> & {
  children?: ReactNode;
  // 允许滚动的方向；另一方向裁掉，不出现滚动条
  orientation?: Orientation;
  gutter?: "overlay" | "stable";
  viewportClassName?: string;
  viewportRef?: Ref<HTMLDivElement>;
  // 视口上的其他属性（aria-label、role、onScroll 等）
  viewportProps?: Omit<ComponentProps<"div">, "ref" | "className" | "children">;
  // 视口改用别的元素渲染（例如 Select.List，让它自己就是滚动元素）
  viewportRender?: ReactElement;
  contentClassName?: string;
  // 内容层改用别的元素渲染（例如 <ul />，子元素是 <li>）；false 表示不要内容层
  contentRender?: ReactElement | false;
};

const overflowFor: Record<Orientation, CSSProperties> = {
  vertical: { overflowX: "hidden", overflowY: "scroll" },
  horizontal: { overflowX: "scroll", overflowY: "hidden" },
  both: { overflow: "scroll" },
};

function ScrollArea({
  className,
  children,
  orientation = "vertical",
  gutter = "overlay",
  viewportClassName,
  viewportRef,
  viewportProps,
  viewportRender,
  contentClassName,
  contentRender,
  ...props
}: ScrollAreaProps) {
  const { style: viewportStyle, ...restViewport } = viewportProps ?? {};
  return (
    <ScrollAreaPrimitive.Root data-slot="scroll-area" data-orientation={orientation} data-gutter={gutter} className={cn("weir-scroll-root", className)} {...props}>
      <ScrollAreaPrimitive.Viewport
        data-slot="scroll-area-viewport"
        ref={viewportRef}
        render={viewportRender}
        className={cn("weir-scroll-viewport", viewportClassName)}
        style={{ ...overflowFor[orientation], ...viewportStyle }}
        {...restViewport}
      >
        {contentRender === false ? (
          children
        ) : (
          // 内容层只用于测量内容尺寸变化（新增行时拇指长度跟着变），不改变排版；
          // Base UI 默认给它 min-width: fit-content，纵向滚动时去掉，避免宽内容把视口撑出横向滚动
          <ScrollAreaPrimitive.Content
            data-slot="scroll-area-content"
            render={contentRender}
            className={cn("weir-scroll-content", contentClassName)}
            style={orientation === "vertical" ? { minWidth: 0 } : undefined}
          >
            {children}
          </ScrollAreaPrimitive.Content>
        )}
      </ScrollAreaPrimitive.Viewport>
      {orientation !== "horizontal" && <ScrollBar orientation="vertical" />}
      {orientation !== "vertical" && <ScrollBar orientation="horizontal" />}
      {orientation === "both" && <ScrollAreaPrimitive.Corner className="weir-scroll-corner" />}
    </ScrollAreaPrimitive.Root>
  );
}

function ScrollBar({ orientation }: { orientation: "vertical" | "horizontal" }) {
  return (
    <ScrollAreaPrimitive.Scrollbar data-slot="scroll-area-scrollbar" orientation={orientation} className="weir-scrollbar">
      <ScrollAreaPrimitive.Thumb data-slot="scroll-area-thumb" className="weir-scroll-thumb" />
    </ScrollAreaPrimitive.Scrollbar>
  );
}

export { ScrollArea };
export type { ScrollAreaProps };
