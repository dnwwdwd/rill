import { useEffect, useRef, type ComponentType } from 'react';
import { createPortal } from 'react-dom';
import { IconButton } from '@/components/ui';
import { SettingsNav } from '@/components/settings/SettingsNav';
import { SourcesSettings } from '@/components/settings/SourcesSettings';
import { ReadingSettings } from '@/components/settings/ReadingSettings';
import { AiSettings } from '@/components/settings/AiSettings';
import { AutomationSettings } from '@/components/settings/AutomationSettings';
import { McpSettings } from '@/components/settings/McpSettings';
import { BackupSettings } from '@/components/settings/BackupSettings';
import { SecuritySettings } from '@/components/settings/SecuritySettings';
import { useUiStore } from '@/stores/uiStore';
import type { SettingsSection } from '@/types/ui';

const sections: Record<SettingsSection, ComponentType> = {
  sources: SourcesSettings,
  reading: ReadingSettings,
  ai: AiSettings,
  automation: AutomationSettings,
  mcp: McpSettings,
  backup: BackupSettings,
  security: SecuritySettings,
};

// REQ-023：无标题栏弹窗 —— 左列「设置」字标 + 目录，关闭按钮悬浮于内容区右上。
export const SettingsDialog = () => {
  const open = useUiStore((state) => state.settingsOpen);
  const section = useUiStore((state) => state.settingsSection);
  const close = useUiStore((state) => state.closeSettings);
  const setSection = useUiStore((state) => state.setSettingsSection);
  const dialogRef = useRef<HTMLElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const backdropRef = useRef<HTMLDivElement>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (contentRef.current) contentRef.current.scrollTop = 0;
  }, [section]);

  useEffect(() => {
    if (!open) return undefined;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      // 让添加源等通用弹层先处理 Esc，避免一次关闭两层。
      if (document.querySelector('.modal-backdrop')) return;
      close();
    };
    const previousOverflow = document.body.style.overflow;
    previousFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    document.body.style.overflow = 'hidden';
    // 设置弹窗内没有可滚动祖先的区域，滚轮事件会继续寻找页面上的滚动容器，
    // 导致背后的文章详情跟着滚动。这里在捕获阶段拦截：只有设置弹窗内部真正
    // 可滚动的元素（内容区、范围树等）才允许滚动，到达边界后也不再下传。
    const onWheel = (event: WheelEvent) => {
      const backdrop = backdropRef.current;
      if (!backdrop) return;
      const target = event.target;
      if (!(target instanceof Element)) {
        event.preventDefault();
        return;
      }
      // 添加源等通用弹层是独立 portal，不应被设置弹窗阻断。
      if (target.closest('.modal-backdrop')) return;
      let node: Element | null = target;
      let scroller: HTMLElement | null = null;
      while (node && node !== backdrop) {
        if (node instanceof HTMLElement) {
          const style = window.getComputedStyle(node);
          if (/(auto|scroll)/.test(style.overflowY) && node.scrollHeight > node.clientHeight) {
            scroller = node;
            break;
          }
        }
        node = node.parentElement;
      }
      if (!scroller) {
        event.preventDefault();
        return;
      }
      const atTop = scroller.scrollTop <= 0;
      const atBottom = scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - 1;
      if ((event.deltaY < 0 && atTop) || (event.deltaY > 0 && atBottom)) {
        event.preventDefault();
      }
    };
    window.addEventListener('wheel', onWheel, { passive: false, capture: true });
    window.addEventListener('keydown', onKey);
    dialogRef.current?.focus();
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener('wheel', onWheel, true);
      window.removeEventListener('keydown', onKey);
      previousFocusRef.current?.focus();
    };
  }, [open, close]);

  if (!open) return null;
  const Section = sections[section];

  return createPortal(
    <div
      ref={backdropRef}
      className="st-backdrop"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) close();
      }}
    >
      <section
        ref={dialogRef}
        className="st-dialog"
        role="dialog"
        aria-modal="true"
        aria-label="设置"
        tabIndex={-1}
      >
        <aside className="st-side">
          <div className="st-brand">设置</div>
          <SettingsNav active={section} onSelect={setSection} />
        </aside>
        <div className="st-main">
          <IconButton className="st-close" icon="x" label="关闭设置" onClick={close} />
          <div className="st-scroll" ref={contentRef}>
            <div className="st-content">
              <Section />
            </div>
          </div>
        </div>
      </section>
    </div>,
    document.body,
  );
};
