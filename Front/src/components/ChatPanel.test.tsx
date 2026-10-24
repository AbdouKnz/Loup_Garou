// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@testing-library/react';

import ChatPanel from './ChatPanel';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  delete (Element.prototype as { scrollIntoView?: unknown }).scrollIntoView;
});

describe('ChatPanel', () => {
  it('fait défiler le journal sans déplacer la page', () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    const scrollHeight = vi.spyOn(Element.prototype, 'scrollHeight', 'get').mockReturnValue(420);

    const { container } = render(
      <ChatPanel
        messages={[
          { id: 'm1', pseudo: 'Alice', text: 'Bonjour', at: '2026-10-19T20:00:00Z' },
          { id: 'm2', pseudo: 'Bob', text: 'Bonsoir', at: '2026-10-19T20:01:00Z' },
        ]}
        onSend={() => {}}
        disabled={false}
      />,
    );

    const log = container.querySelector('.chat-log');
    expect(log).not.toBeNull();
    expect(log?.scrollTop).toBe(420);
    expect(scrollIntoView).not.toHaveBeenCalled();

    scrollHeight.mockRestore();
  });
});
