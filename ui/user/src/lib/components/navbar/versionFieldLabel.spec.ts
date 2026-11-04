import { versionFieldLabel } from './versionFieldLabel';
import { describe, expect, it } from 'vitest';

describe('versionFieldLabel', () => {
	it('maps the obot API field to the uni brand label', () => {
		expect(versionFieldLabel('obot')).toBe('uni');
	});

	it('strips the github.com prefix and keeps unknown keys as-is', () => {
		expect(versionFieldLabel('github.com/obot-platform/obot')).toBe('obot-platform/obot');
		expect(versionFieldLabel('nanobot')).toBe('nanobot');
	});
});
