import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { mockApi } from '@/api/mock/mockApi';
import { AddSourceDialog, bskyHandleFromInput, bskyRssUrl, xHandleFromInput } from '@/components/sources/AddSourceDialog';
import { useSourcesStore } from '@/stores/sourcesStore';

describe('xHandleFromInput', () => {
  it('recognizes X profile links and handles', () => {
    expect(xHandleFromInput('@example_user')).toBe('example_user');
    expect(xHandleFromInput('example_user')).toBe('example_user');
    expect(xHandleFromInput('https://x.com/example_user')).toBe('example_user');
    expect(xHandleFromInput('x.com/example_user/status/123')).toBe('example_user');
    expect(xHandleFromInput('https://mobile.twitter.com/example_user/')).toBe('example_user');
  });

  it('leaves normal feed and site URLs to discovery', () => {
    expect(xHandleFromInput('https://example.com/feed.xml')).toBeNull();
    expect(xHandleFromInput('example.com')).toBeNull();
    expect(xHandleFromInput('@bad-handle!')).toBeNull();
    expect(xHandleFromInput('2803301701')).toBeNull();
    expect(xHandleFromInput('5ed33dcc418a84a046f579ec')).toBeNull();
    expect(xHandleFromInput('bestblogs:OpenAI')).toBeNull();
    expect(xHandleFromInput('')).toBeNull();
  });
});

describe('bskyHandleFromInput', () => {
  it('recognizes Bluesky profile links and dotted handles', () => {
    expect(bskyHandleFromInput('https://bsky.app/profile/bsky.app')).toBe('bsky.app');
    expect(bskyHandleFromInput('bsky.app/profile/alice.example.com/rss')).toBe('alice.example.com');
    expect(bskyHandleFromInput('@alice.bsky.social')).toBe('alice.bsky.social');
    expect(bskyHandleFromInput('https://bsky.app/profile/alice.test/profile-dialog')).toBe('alice.test');
  });

  it('rejects non-Bluesky input', () => {
    expect(bskyHandleFromInput('example_user')).toBeNull();
    expect(bskyHandleFromInput('@twitter-style')).toBeNull();
    expect(bskyHandleFromInput('https://example.com/profile/alice')).toBeNull();
    expect(bskyHandleFromInput('bestblogs:OpenAI')).toBeNull();
    expect(bskyHandleFromInput('')).toBeNull();
  });

  it('builds the official RSS url', () => {
    expect(bskyRssUrl('alice.bsky.social')).toBe('https://bsky.app/profile/alice.bsky.social/rss');
  });
});

afterEach(() => {
  act(() => {
    useSourcesStore.setState({ folders: [], sources: [], loaded: false, health: {} });
  });
});

describe('creator search mode', () => {
  it('searches and batch subscribes selected candidates', async () => {
    await mockApi.signIn?.();
    useSourcesStore.setState({ folders: [], sources: [], loaded: true });
    const onClose = vi.fn();
    render(<AddSourceDialog open onClose={onClose} />);

    fireEvent.click(screen.getByRole('button', { name: '搜索创作者' }));
    fireEvent.change(screen.getByLabelText('创作者名称'), { target: { value: '这集我看过' } });
    fireEvent.click(screen.getByRole('button', { name: '搜索' }));

    const candidates = await screen.findAllByRole('button', { name: /这集我看过/ });
    expect(candidates.length).toBeGreaterThan(0);
    fireEvent.click(candidates[0]);
    const subscribe = screen.getByRole('button', { name: '订阅选中 (1)' });
    expect(subscribe).toBeEnabled();
    fireEvent.click(subscribe);

    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });
});
