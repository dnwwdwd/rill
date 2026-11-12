import { useEffect, useRef, useState } from 'react';

export const useSpeech = (text: string) => {
  const [speaking, setSpeaking] = useState(false);
  const [paused, setPaused] = useState(false);
  const utteranceRef = useRef<SpeechSynthesisUtterance | null>(null);
  const supported = typeof window !== 'undefined' && 'speechSynthesis' in window;

  const stop = () => {
    if (supported) window.speechSynthesis.cancel();
    utteranceRef.current = null;
    setSpeaking(false);
    setPaused(false);
  };

  useEffect(() => stop, [text]);

  const start = () => {
    if (!supported || !text.trim()) return;
    stop();
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = /[\u4e00-\u9fff]/.test(text.slice(0, 200)) ? 'zh-CN' : 'en-US';
    utterance.rate = 1;
    utterance.onend = () => {
      utteranceRef.current = null;
      setSpeaking(false);
      setPaused(false);
    };
    utterance.onerror = () => {
      utteranceRef.current = null;
      setSpeaking(false);
      setPaused(false);
    };
    utteranceRef.current = utterance;
    window.speechSynthesis.speak(utterance);
    setSpeaking(true);
    setPaused(false);
  };

  const toggle = () => {
    if (!supported) return;
    if (!speaking) {
      start();
      return;
    }
    if (paused) {
      window.speechSynthesis.resume();
      setPaused(false);
    } else {
      window.speechSynthesis.pause();
      setPaused(true);
    }
  };

  return { supported, speaking, paused, toggle, stop };
};
