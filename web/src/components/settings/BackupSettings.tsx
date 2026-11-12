import { useRef } from 'react';
import { Button, Switch } from '@/components/ui';
import { RowGroup, Row } from '@/components/settings/SettingsSection';
import { useSettingsStore } from '@/stores/settingsStore';
import { useUiStore } from '@/stores/uiStore';
import { formatAgo, formatDateTime } from '@/utils/date';

// 备份策略（2026-11-02）：每日自动备份仍写入应用数据目录 backups/；
// 手动「下载备份」直接把数据库文件下载到本地（懒猫 file chooser 拦截接管保存），
// 不占用自动备份份数。
export const BackupSettings = () => {
  const settings = useSettingsStore((state) => state.settings);
  const update = useSettingsStore((state) => state.update);
  const downloadBackup = useSettingsStore((state) => state.downloadBackup);
  const restoreBackup = useSettingsStore((state) => state.restoreBackup);
  const notify = useUiStore((state) => state.notify);
  const fileRef = useRef<HTMLInputElement>(null);
  const backup = settings.backup;

  const onFile = (file: File | undefined) => {
    if (!file) return;
    void restoreBackup(file);
    if (fileRef.current) fileRef.current.value = '';
  };

  const download = async () => {
    try {
      await downloadBackup();
      notify('备份文件已开始下载');
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : '备份下载失败');
    }
  };

  return (
    <>
      <RowGroup title="自动备份">
        <Row
          label="每日自动备份"
          control={(
            <Switch
              checked={backup.daily}
              onChange={(value) => update({ backup: { daily: value } })}
              label="每日自动备份"
            />
          )}
        />
        <Row
          label="保留最近"
          hint="每天自动备份一份完整数据库到应用数据目录 backups/；超过保留份数的旧备份自动删除。手动下载的文件不占份数。"
          control={(
            <select
              value={backup.keep}
              onChange={(event) => update({ backup: { keep: Number(event.target.value) } })}
              aria-label="备份保留份数"
            >
              {[3, 7, 14, 30].map((value) => (
                <option value={value} key={value}>最近 {value} 份</option>
              ))}
            </select>
          )}
        />
        <Row
          label="最近备份"
          control={<span className="st-dim">{backup.last_backup_at ? formatAgo(backup.last_backup_at) : '还没有备份'}</span>}
          hint={backup.last_backup_at ? formatDateTime(backup.last_backup_at) : undefined}
        />
      </RowGroup>

      <RowGroup title="手动操作" tone="danger">
        <Row
          label="下载备份 / 从备份恢复"
          hint="下载备份会把当前数据库文件保存到本地；从备份恢复会替换当前数据库，恢复期间应用短暂不可用。"
          control={(
            <div className="st-inline">
              <Button size="sm" variant="primary" icon="download" onClick={() => { void download(); }}>下载备份</Button>
              <Button size="sm" variant="danger" icon="upload" onClick={() => fileRef.current?.click()}>从备份恢复</Button>
            </div>
          )}
        />
        <input
          ref={fileRef}
          type="file"
          accept=".db,.sqlite,.sqlite3,application/octet-stream"
          hidden
          onChange={(event) => onFile(event.target.files?.[0])}
        />
      </RowGroup>

      <RowGroup title="备份范围">
        <Row className="st-facts-row">
          <ul className="st-facts">
            <li>订阅源、文件夹、条目、阅读与收藏状态、抓取日志和偏好设置。</li>
            <li>AI 密钥以加密形式保存；媒体缓存不进入备份。</li>
            <li>OPML 只包含订阅结构，不包含阅读状态与 AI 结果。</li>
          </ul>
        </Row>
      </RowGroup>
    </>
  );
};
