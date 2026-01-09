import {
	register,
	init,
	getLocaleFromNavigator,
	locale as $locale,
	_ as $t,
	waitLocale
} from 'svelte-i18n';

// Register locale loaders with dynamic imports for code splitting
register('en', () => import('./locales/en.json'));
register('zh-CN', () => import('./locales/zh-CN.json'));

// Track if i18n has been initialized
let initialized = false;

/**
 * Initialize the i18n system synchronously (for SSR compatibility)
 * This must be called before any component uses $t()
 */
export function setupI18n(initialLocale?: string): void {
	if (initialized) return;
	initialized = true;

	init({
		fallbackLocale: 'en',
		initialLocale: initialLocale || getInitialLocale()
	});
}

/**
 * Get initial locale - works on both server and client
 */
function getInitialLocale(): string {
	// On server, use fallback
	if (typeof window === 'undefined') {
		return 'en';
	}

	// On client, check localStorage first
	const savedLocale = window.localStorage.getItem('obot_locale');
	if (savedLocale && isValidLocale(savedLocale)) {
		return savedLocale;
	}

	// Then check browser preference
	const browserLocale = getLocaleFromNavigator() || 'en';
	if (browserLocale.toLowerCase().startsWith('zh')) {
		return 'zh-CN';
	}

	return 'en';
}

/**
 * Validate locale string
 */
function isValidLocale(locale: string): boolean {
	return ['en', 'zh-CN'].includes(locale);
}

/**
 * Set locale and persist to localStorage
 */
export function setLocale(newLocale: string): void {
	if (!isValidLocale(newLocale)) {
		console.warn(`Invalid locale: ${newLocale}, falling back to 'en'`);
		newLocale = 'en';
	}

	$locale.set(newLocale);

	if (typeof window !== 'undefined') {
		window.localStorage.setItem('obot_locale', newLocale);
	}
}

// Re-export svelte-i18n stores for component usage
export const locale = $locale;
export const t = $t;
export { waitLocale };

// Initialize immediately for SSR compatibility
setupI18n();
