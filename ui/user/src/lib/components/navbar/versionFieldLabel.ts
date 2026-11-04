const VERSION_FIELD_LABELS: Record<string, string> = {
	obot: 'uni'
};

export function versionFieldLabel(key: string): string {
	return (VERSION_FIELD_LABELS[key] ?? key).replace('github.com/', '');
}
