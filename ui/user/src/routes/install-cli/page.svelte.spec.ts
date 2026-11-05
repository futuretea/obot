import { preparePageData } from '../../tests/helpers/pageData';
import InstallCliPage from './+page.svelte';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';

// The page builds the setup command from $app/state page.url; the vitest browser
// runner's placeholder URL has the string "null" as its origin.
vi.mock('$app/state', () => ({
	page: {
		url: new URL('https://uni.internal.example/install-cli'),
		data: {},
		form: null,
		error: null,
		params: {},
		route: { id: null },
		state: {},
		status: 200
	},
	navigating: {
		from: null,
		to: null,
		type: null,
		willUnload: null,
		delta: null,
		complete: null
	},
	updated: { current: false, check: async () => false }
}));

const goreleaserArchives = [
	'uni_v0.26.3-uni_darwin_arm64.tar.gz',
	'uni_v0.26.3-uni_darwin_amd64.tar.gz',
	'uni_v0.26.3-uni_linux_amd64.tar.gz',
	'uni_v0.26.3-uni_linux_arm64.tar.gz',
	'uni_v0.26.3-uni_windows_amd64.zip',
	'uni_v0.26.3-uni_windows_arm64.zip'
];

const installToPath = 'sudo install -m 0755 uni /usr/local/bin/uni';

function archiveCommand(os: string, arch: string) {
	const archive = `uni_v0.26.3-uni_${os}_${arch}.tar.gz`;
	return new RegExp(
		`curl -LO \\S+/${archive.replace(/\./g, '\\.')} && tar xzf ${archive.replace(/\./g, '\\.')} && ${installToPath.replace(/[/.]/g, '\\$&')} && uni --help`
	);
}

const downloadBaseURL = 'https://downloads.example.com/releases/v0.26.3-uni';

async function renderPage(cliDownloadBaseURL: string | undefined = downloadBaseURL) {
	await preparePageData({ version: { cliDownloadBaseURL } });
	return render(InstallCliPage);
}

describe('install-cli page', () => {
	it('links archives using the GoReleaser naming scheme', async () => {
		await renderPage();
		for (const name of goreleaserArchives) {
			await expect.element(page.getByRole('link', { name })).toBeInTheDocument();
		}
	});

	it('provides a checksums.txt download for integrity verification', async () => {
		await renderPage();
		await expect.element(page.getByRole('link', { name: 'checksums.txt' })).toBeInTheDocument();
	});

	it.each([downloadBaseURL, `${downloadBaseURL}/`])(
		'uses the configured download directory %s for every link and command',
		async (baseURL) => {
			await renderPage(baseURL);
			for (const file of [...goreleaserArchives, 'checksums.txt']) {
				await expect
					.element(page.getByRole('link', { name: file }))
					.toHaveAttribute('href', `${downloadBaseURL}/${file}`);
			}
			for (const file of goreleaserArchives.filter((file) => file.endsWith('.tar.gz'))) {
				await expect
					.element(page.getByText(`curl -LO '${downloadBaseURL}'/${file}`, { exact: false }))
					.toBeInTheDocument();
			}
			await expect
				.element(page.getByText('CLI download URL is not configured.'))
				.not.toBeInTheDocument();
		}
	);

	it.each(['', '   '])(
		'shows the unconfigured state for %j without hiding usage',
		async (baseURL) => {
			await renderPage(baseURL);
			await expect
				.element(page.getByText('CLI download URL is not configured.'))
				.toBeInTheDocument();
			for (const file of [...goreleaserArchives, 'checksums.txt']) {
				await expect.element(page.getByRole('link', { name: file })).not.toBeInTheDocument();
			}
			await expect.element(page.getByText(/curl -LO/)).not.toBeInTheDocument();
			await expect
				.element(page.getByText('uni setup --url https://uni.internal.example'))
				.toBeInTheDocument();
			await expect
				.element(page.getByRole('heading', { name: 'uni CLI Commands' }))
				.toBeInTheDocument();
		}
	);

	it('supports a version response without download configuration', async () => {
		await preparePageData({ version: {} });
		render(InstallCliPage);
		await expect.element(page.getByText('CLI download URL is not configured.')).toBeInTheDocument();
		await expect.element(page.getByText(/curl -LO/)).not.toBeInTheDocument();
	});

	it('quotes shell metacharacters in the configured download directory', async () => {
		await renderPage("https://downloads.example.com/uni's$builds");
		await expect
			.element(
				page.getByText(
					String.raw`curl -LO 'https://downloads.example.com/uni'\''s$builds'/uni_v0.26.3-uni_darwin_arm64.tar.gz`,
					{ exact: false }
				)
			)
			.toBeInTheDocument();
	});

	it.each([
		['darwin', 'arm64'],
		['darwin', 'amd64'],
		['linux', 'amd64'],
		['linux', 'arm64']
	])(
		'installs uni onto PATH for %s/%s with a matching download and unpack step',
		async (os, arch) => {
			await renderPage();
			await expect.element(page.getByText(archiveCommand(os, arch))).toBeInTheDocument();
		}
	);

	it('guides Windows users through unpacking, verifying, and running', async () => {
		await renderPage();
		await expect
			.element(
				page.getByText(
					/Expand-Archive \.\\uni_v0\.26\.3-uni_windows_amd64\.zip -DestinationPath \.; \.\\uni\.exe --help/
				)
			)
			.toBeInTheDocument();
		await expect
			.element(page.getByText(String.raw`.\uni.exe setup --url https://uni.internal.example`))
			.toBeInTheDocument();
		await expect.element(page.getByText(/Get-FileHash/)).toBeInTheDocument();
	});

	it('shows a no-sudo install alternative that creates its target directory', async () => {
		await renderPage();
		await expect
			.element(
				page.getByText(/mkdir -p ~\/\.local\/bin && install -m 0755 uni ~\/\.local\/bin\/uni/)
			)
			.toBeInTheDocument();
	});

	it('shows the setup command and accurate setup capabilities', async () => {
		await renderPage();
		await expect
			.element(page.getByText('uni setup --url https://uni.internal.example'))
			.toBeInTheDocument();
		await expect.element(page.getByText('Detects Claude Code on your machine')).toBeInTheDocument();
	});

	it('does not claim Cursor detection the CLI does not provide', async () => {
		await renderPage();
		await expect.element(page.getByText(/Detects Cursor/)).not.toBeInTheDocument();
		await expect.element(page.getByText(/Connected Cursor/)).not.toBeInTheDocument();
	});

	it('does not claim uni mcp installs MCP servers', async () => {
		await renderPage();
		await expect.element(page.getByText(/install and manage MCP servers/)).not.toBeInTheDocument();
	});
});
