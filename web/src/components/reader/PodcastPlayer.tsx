import { useEffect, useRef, useState } from 'react';
import { api } from '@/api/client';
import { Icon } from '@/components/ui';

interface PodcastPlayerProps {
  itemId: number;
  src: string;
  type?: string;
  initialPosition?: number;
}

const SPEEDS = [1, 1.25, 1.5, 2, 0.75];

const formatTime = (seconds: number) => {
  if (!Number.isFinite(seconds) || seconds < 0) return '--:--';
  const total = Math.floor(seconds);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  const mm = String(minutes).padStart(2, '0');
  const ss = String(secs).padStart(2, '0');
  return hours > 0 ? `${hours}:${mm}:${ss}` : `${minutes}:${ss}`;
};

export const PodcastPlayer = ({ itemId, src, type, initialPosition = 0 }: PodcastPlayerProps) => {
  const audioRef = useRef<HTMLAudioElement>(null);
  const savedAtRef = useRef(0);
  const lastSyncedRef = useRef(0);
  const finishedRef = useRef(false);
  const [playing, setPlaying] = useState(false);
  const [current, setCurrent] = useState(0);
  const [duration, setDuration] = useState(0);
  const [speed, setSpeed] = useState(1);
  const [error, setError] = useState(false);
  const storageKey = `rill.podcast.position.${itemId}`;

  useEffect(() => {
    setPlaying(false);
    setCurrent(0);
    setDuration(0);
    setSpeed(1);
    setError(false);
    savedAtRef.current = 0;
    lastSyncedRef.current = 0;
    finishedRef.current = false;
  }, [itemId, src]);

  const syncProgress = (position: number, total: number, force = false) => {
    if (!Number.isFinite(position) || position < 0) return;
    const now = Date.now();
    if (!force && now - lastSyncedRef.current < 5000) return;
    lastSyncedRef.current = now;
    void api.savePodcastProgress(itemId, Math.round(position), Math.round(Number.isFinite(total) ? total : 0))
      .catch(() => {
        /* 进度同步失败不打断播放，本地副本仍保留 */
      });
  };

  useEffect(() => {
    const audio = audioRef.current;
    return () => {
      if (finishedRef.current) return;
      if (audio && audio.currentTime > 0) {
        window.localStorage.setItem(storageKey, String(Math.round(audio.currentTime)));
        void api.savePodcastProgress(itemId, Math.round(audio.currentTime), Math.round(audio.duration || 0))
          .catch(() => {
            /* ignore unmount sync failure */
          });
      }
    };
  }, [itemId, storageKey]);

  useEffect(() => {
    if (audioRef.current) audioRef.current.playbackRate = speed;
  }, [speed]);

  const toggle = async () => {
    const audio = audioRef.current;
    if (!audio) return;
    if (audio.paused) {
      try {
        await audio.play();
      } catch {
        setError(true);
      }
    } else {
      audio.pause();
    }
  };

  const seek = (value: number) => {
    const audio = audioRef.current;
    if (!audio) return;
    audio.currentTime = value;
    setCurrent(value);
  };

  const skip = (delta: number) => {
    const audio = audioRef.current;
    if (!audio) return;
    const limit = Number.isFinite(audio.duration) ? audio.duration : Math.max(current + delta, 0);
    seek(Math.min(Math.max(audio.currentTime + delta, 0), limit));
  };

  const cycleSpeed = () => {
    const index = SPEEDS.indexOf(speed);
    setSpeed(SPEEDS[(index + 1) % SPEEDS.length]);
  };

  const onTimeUpdate = () => {
    const audio = audioRef.current;
    if (!audio) return;
    setCurrent(audio.currentTime);
    if (Math.abs(audio.currentTime - savedAtRef.current) >= 5) {
      savedAtRef.current = audio.currentTime;
      window.localStorage.setItem(storageKey, String(Math.round(audio.currentTime)));
    }
    syncProgress(audio.currentTime, audio.duration);
  };

  const onLoadedMetadata = () => {
    const audio = audioRef.current;
    if (!audio) return;
    const total = audio.duration || 0;
    setDuration(total);
    const localSaved = Number(window.localStorage.getItem(storageKey) ?? 0);
    const saved = initialPosition > 5 ? initialPosition : localSaved;
    if (Number.isFinite(saved) && saved > 5 && saved < total - 5) {
      audio.currentTime = saved;
      setCurrent(saved);
    }
  };

  return (
    <section className="podcast-player" aria-label="播客播放器">
      <audio
        ref={audioRef}
        src={src}
        preload="metadata"
        onPlay={() => setPlaying(true)}
        onPause={() => {
          setPlaying(false);
          const audio = audioRef.current;
          if (audio && audio.currentTime > 0) syncProgress(audio.currentTime, audio.duration, true);
        }}
        onTimeUpdate={onTimeUpdate}
        onLoadedMetadata={onLoadedMetadata}
        onEnded={() => {
          finishedRef.current = true;
          setPlaying(false);
          window.localStorage.removeItem(storageKey);
          syncProgress(0, duration, true);
        }}
        onError={() => setError(true)}
      />
      <div className="podcast-controls">
        <button
          type="button"
          className="podcast-play"
          onClick={() => { void toggle(); }}
          aria-label={playing ? '暂停' : '播放'}
          title={playing ? '暂停' : '播放'}
        >
          <Icon name={playing ? 'pause' : 'play'} size={20} />
        </button>
        <div className="podcast-timeline">
          <input
            type="range"
            min={0}
            max={Math.max(duration, 0)}
            step={1}
            value={Math.min(current, duration || 0)}
            onChange={(event) => seek(Number(event.target.value))}
            aria-label="播放进度"
          />
          <div className="podcast-time">
            <span>{formatTime(current)}</span>
            <span>{duration ? formatTime(duration) : '--:--'}</span>
          </div>
        </div>
        <button
          type="button"
          className="podcast-speed"
          onClick={cycleSpeed}
          aria-label={`播放速度 ${speed} 倍`}
          title="切换播放速度"
        >
          {speed}×
        </button>
      </div>
      <div className="podcast-actions">
        <button type="button" className="podcast-skip" onClick={() => skip(-15)} aria-label="后退 15 秒">
          −15s
        </button>
        <button type="button" className="podcast-skip" onClick={() => skip(15)} aria-label="前进 15 秒">
          +15s
        </button>
        <span className={`podcast-status${error ? ' is-error' : ''}`}>
          {error ? '音频无法播放' : type?.startsWith('audio/') ? '播客' : '音频'}
        </span>
      </div>
    </section>
  );
};
