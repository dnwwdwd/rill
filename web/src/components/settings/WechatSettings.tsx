import { useEffect, useRef, useState } from 'react';
import { isMockMode } from '@/api/client';
import {
  Button,
  Icon,
  Switch,
} from '@/components/ui';
import { RowGroup, Row } from '@/components/settings/SettingsSection';
import { useSettingsStore } from '@/stores/settingsStore';
import { useUiStore } from '@/stores/uiStore';
import { cx } from '@/utils/cx';

const QR_SIZE = 21;
const QR_POLL_INTERVAL = 3000;
// 后端二维码 TTL 为 5 分钟，前端轮询略晚于它超时，保证能看到后端的过期状态。
const QR_TIMEOUT = 5.5 * 60 * 1000;

const qrDark = (row: number, col: number): boolean => {
  const finder = (r: number, c: number) => {
    if (r < 0 || c < 0 || r > 6 || c > 6) return null;
    const border = r === 0 || c === 0 || r === 6 || c === 6;
    const core = r >= 2 && r <= 4 && c >= 2 && c <= 4;
    return border || core;
  };
  const topLeft = finder(row, col);
  if (topLeft !== null) return topLeft;
  const topRight = finder(row, col - 14);
  if (topRight !== null) return topRight;
  const bottomLeft = finder(row - 14, col);
  if (bottomLeft !== null) return bottomLeft;
  return (row * 31 + col * 17 + row * col * 7) % 11 < 5;
};

// 仅 mock 模式使用：演示用占位二维码，不接触任何真实接口。
const QrPlaceholder = ({ connected, disabled, onScan }: { connected: boolean; disabled: boolean; onScan: () => void }) => {
  const cells = [];
  for (let row = 0; row < QR_SIZE; row += 1) {
    for (let col = 0; col < QR_SIZE; col += 1) {
      cells.push(
        <span
          key={`${row}-${col}`}
          className={cx('qr-cell', qrDark(row, col) && 'is-dark')}
        />,
      );
    }
  }
  return (
    <button
      type="button"
      className={cx('st-qr-frame', connected && 'is-scanned', disabled && 'is-disabled')}
      onClick={onScan}
      disabled={disabled}
      aria-label={connected ? '微信读书已连接，点击重新模拟扫码' : '点击二维码模拟扫码连接微信读书'}
    >
      <span className="qr-grid" aria-hidden="true">{cells}</span>
      {connected ? (
        <span className="st-qr-overlay">
          <Icon name="check-circle" size={24} />
          <strong>已连接</strong>
        </span>
      ) : null}
    </button>
  );
};

/**
 * REQ-034 / DEC-024：公众号接入只剩微信读书扫码一条通道。
 * 扫码行直接展开二维码，不再依赖外部登录态告警。
 */
export const WechatChannel = () => {
  const settings = useSettingsStore((state) => state.settings);
  const setWechatEnabled = useSettingsStore((state) => state.setWechatEnabled);
  const startWereadScan = useSettingsStore((state) => state.startWereadScan);
  const pollWereadScanStatus = useSettingsStore((state) => state.pollWereadScanStatus);
  const scanWeread = useSettingsStore((state) => state.scanWeread);
  const notify = useUiStore((state) => state.notify);
  const wechat = settings.wechat;
  const wereadConnected = wechat.weread_connected;

  const [wereadOpen, setWereadOpen] = useState(false);
  const [wereadQrcode, setWereadQrcode] = useState('');
  const [wereadPolling, setWereadPolling] = useState(false);
  const [wereadError, setWereadError] = useState('');
  const pollFailures = useRef(0);

  // 微信读书扫码轮询：成功后 store 会刷新设置（weread_connected → 已连接）并提示，
  // 这里只负责收尾，避免停在扫码界面。
  useEffect(() => {
    if (!wereadPolling) return undefined;
    const started = Date.now();
    const timer = window.setInterval(() => {
      void (async () => {
        try {
          const status = await pollWereadScanStatus();
          pollFailures.current = 0;
          setWereadError('');
          if (status.state === 'done') {
            setWereadPolling(false);
            setWereadQrcode('');
            setWereadOpen(false);
            return;
          }
          if (status.state === 'expired' || status.state === 'error') {
            setWereadPolling(false);
            setWereadQrcode('');
            setWereadError(status.message || (status.state === 'expired' ? '二维码已过期，请重新获取' : '微信读书扫码失败'));
            return;
          }
        } catch (error) {
          // 后端/微信读书暂时不可达时继续轮询，连续失败两次才展示原因。
          pollFailures.current += 1;
          if (pollFailures.current >= 2) {
            setWereadError(error instanceof Error && error.message ? error.message : '微信读书扫码状态检查失败，正在重试…');
          }
        }
        if (Date.now() - started > QR_TIMEOUT) {
          setWereadPolling(false);
          setWereadQrcode('');
          setWereadError('微信读书二维码已超时，请重新获取');
        }
      })();
    }, QR_POLL_INTERVAL);
    return () => {
      pollFailures.current = 0;
      window.clearInterval(timer);
    };
  }, [wereadPolling, pollWereadScanStatus]);

  const startWeread = async () => {
    if (!wechat.enabled) {
      notify('先启用公众号采集，再扫码连接微信读书');
      return;
    }
    setWereadError('');
    // 先展开二维码行：请求失败时也能看到错误，而不是「点了没反应」。
    setWereadOpen(true);
    try {
      const image = await startWereadScan();
      setWereadQrcode(image);
      setWereadPolling(true);
    } catch (error) {
      setWereadError(error instanceof Error ? error.message : '获取微信读书二维码失败');
    }
  };

  return (
    <RowGroup title="公众号接入" className="st-channel">
      <Row
        label="启用公众号采集"
        control={(
          <Switch
            checked={wechat.enabled}
            onChange={(value) => { void setWechatEnabled(value); }}
            label="启用公众号采集"
          />
        )}
      />
      {wechat.enabled ? (
        <>
          <Row
            label="微信读书扫码"
            hint={wereadConnected && wechat.weread_can_renew === false
              ? '当前登录凭据不支持自动续期（缺少 wr_rt），过一会儿还会失效；请重新扫码一次。'
              : undefined}
            control={(
              <div className="st-inline">
                <span className={cx('st-state', wereadConnected ? 'is-ok' : 'is-warn')}>
                  {wereadConnected ? '已连接' : '未连接'}
                </span>
                <Button
                  size="sm"
                  icon="qr"
                  onClick={() => { void startWeread(); }}
                >
                  {wereadOpen ? '刷新二维码' : wereadConnected ? '重新扫码' : '获取二维码'}
                </Button>
              </div>
            )}
          />
          {wereadOpen ? (
            <Row className="st-qr-row">
              <div className="st-qr">
                {isMockMode ? (
                  <QrPlaceholder connected={wereadConnected} disabled={!wechat.enabled} onScan={() => { void scanWeread(); }} />
                ) : wereadQrcode ? (
                  <div className={cx('st-qr-frame', 'is-real', wereadPolling && 'is-polling')}>
                    <img src={wereadQrcode} alt="微信读书登录二维码" className="qr-image" />
                  </div>
                ) : (
                  <button type="button" className="st-qr-frame is-empty" onClick={() => { void startWeread(); }} aria-label="获取微信读书登录二维码">
                    <Icon name="qr" size={26} />
                    <strong>获取二维码</strong>
                  </button>
                )}
                {wereadError ? <span className="form-error">{wereadError}</span> : null}
              </div>
              <div className="st-qr-side">
                <span className="st-qr-note">
                  <Icon name="shield" size={13} /> 微信读书 Cookie 仅加密保存在本机 Rill 数据库；请用微信扫码并在手机确认。
                </span>
              </div>
            </Row>
          ) : null}
        </>
      ) : (
        <Row hint="关闭后公众号源暂停抓取；重新开启并扫码连接微信读书后自动恢复。" />
      )}
    </RowGroup>
  );
};
