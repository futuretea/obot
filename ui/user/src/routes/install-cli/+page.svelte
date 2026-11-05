<script lang="ts">
	import { page } from '$app/state';
	import CopyButton from '$lib/components/CopyButton.svelte';
	import Layout from '$lib/components/Layout.svelte';
	import Logo from '$lib/components/Logo.svelte';
	import { COMMON_AI_CLIENTS } from '$lib/services/user/constants';
	import { darkMode, profile, version } from '$lib/stores';
	import {
		AntennaIcon,
		CheckIcon,
		ComputerIcon,
		LaptopIcon,
		PcCaseIcon,
		ServerIcon
	} from '@lucide/svelte';

	let isAdmin = $derived(profile.current?.hasAdminAccess?.() ?? false);

	const icons = [
		{
			id: 'computer',
			icon: ComputerIcon,
			color: '#4575b4'
		},
		{
			id: 'laptop',
			icon: LaptopIcon,
			color: '#fdae61'
		},
		{
			id: 'pc-case',
			icon: PcCaseIcon,
			color: '#f46d43'
		},
		{
			id: 'server',
			icon: ServerIcon,
			color: '#74add1'
		}
	];

	const marks = COMMON_AI_CLIENTS;
	const downloadBase = $derived(
		version.current.cliDownloadBaseURL?.trim().replace(/\/+$/, '') ?? ''
	);
	const shellDownloadBase = $derived(`'${downloadBase.replaceAll("'", "'\\''")}'`);
	const downloads = [
		{ os: 'macOS (Apple silicon)', file: 'uni_v0.26.3-uni_darwin_arm64.tar.gz' },
		{ os: 'macOS (Intel)', file: 'uni_v0.26.3-uni_darwin_amd64.tar.gz' },
		{ os: 'Linux (x86_64)', file: 'uni_v0.26.3-uni_linux_amd64.tar.gz' },
		{ os: 'Linux (ARM64)', file: 'uni_v0.26.3-uni_linux_arm64.tar.gz' },
		{ os: 'Windows (x86_64)', file: 'uni_v0.26.3-uni_windows_amd64.zip' },
		{ os: 'Windows (ARM64)', file: 'uni_v0.26.3-uni_windows_arm64.zip' },
		{ os: 'SHA-256 checksums', file: 'checksums.txt' }
	];
	const setupCommand = $derived(`uni setup --url ${page.url.origin}`);
	const windowsSetupCommand = $derived(String.raw`.\uni.exe setup --url ${page.url.origin}`);
</script>

<Layout title="uni CLI">
	<div class="w-full max-w-7xl mx-auto h-full @container/cli my-6">
		<section class="paper">
			<div class="flex flex-col @2xl/cli:flex-row items-center justify-center gap-8">
				<div class="max-w-md">
					<Logo class="@2xl/cli:size-16 size-12 mb-1 mx-auto @2xl:mx-0" />
					<h2 class="text-2xl font-bold mb-2">One CLI. Your entire AI tooling.</h2>
					{#if isAdmin}
						<p class="font-light">
							uni is the open-source platform for hosting, governing, and using Model Context
							Protocol servers & skills. Have users run the CLI locally, and bring your team’s AI
							tooling under one roof.
						</p>
					{:else}
						<p class="font-light">
							uni is the open-source platform for hosting, governing, and using Model Context
							Protocol servers & skills. Run the CLI locally to start connecting your AI clients and
							have access to your team's AI tooling in one place.
						</p>
					{/if}
				</div>
				<div class="flex grow shrink-0 justify-end">
					{@render mockupCode()}
				</div>
			</div>
		</section>

		<section class="flex flex-col @3xl/cli:flex-row my-12 items-center justify-center">
			<div class="grid grid-cols-4 gap-4 @2xl/cli:min-w-80">
				{#each icons as icon (icon.id)}
					<div
						class="col-span-2 flex justify-center items-center @2xl/cli:size-36 size-24 rounded-md"
						style={`${darkMode.isDark ? 'background-color: color-mix(in oklab, ' + icon.color + ' 20%, transparent);' : 'background-color: color-mix(in oklab, ' + icon.color + ' 10%, transparent);'} color: ${icon.color};`}
					>
						<icon.icon class="@2xl/cli:size-24 size-16" />
					</div>
				{/each}
			</div>
			<div class="p-8 @2xl/cli:pb-8 pb-0 flex flex-col gap-1" id="uni-cli-installation">
				<h3 class="text-2xl font-bold">How to Install uni CLI</h3>

				{#if downloadBase}
					<p class="text-sm mt-1">Download the archive for your system and unpack it:</p>
					<div class="overflow-x-auto mt-2">
						<table class="table table-sm">
							<tbody>
								{#each downloads as item (item.file)}
									<tr>
										<td class="whitespace-nowrap">{item.os}</td>
										<td>
											<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- configured archive URL, not an in-app route -->
											<a
												class="link link-primary"
												href={`${downloadBase}/${item.file}`}
												target="_blank"
												rel="external noopener noreferrer"
											>
												{item.file}
											</a>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>

					{@render codesnippet(
						'macOS (Apple silicon): download, unpack, install, and verify:',
						`curl -LO ${shellDownloadBase}/uni_v0.26.3-uni_darwin_arm64.tar.gz && tar xzf uni_v0.26.3-uni_darwin_arm64.tar.gz && sudo install -m 0755 uni /usr/local/bin/uni && uni --help`,
						'uni-cli-macos-arm64-install'
					)}

					{@render codesnippet(
						'macOS (Intel): download, unpack, install, and verify:',
						`curl -LO ${shellDownloadBase}/uni_v0.26.3-uni_darwin_amd64.tar.gz && tar xzf uni_v0.26.3-uni_darwin_amd64.tar.gz && sudo install -m 0755 uni /usr/local/bin/uni && uni --help`,
						'uni-cli-macos-amd64-install'
					)}

					{@render codesnippet(
						'Linux (x86_64): download, unpack, install, and verify:',
						`curl -LO ${shellDownloadBase}/uni_v0.26.3-uni_linux_amd64.tar.gz && tar xzf uni_v0.26.3-uni_linux_amd64.tar.gz && sudo install -m 0755 uni /usr/local/bin/uni && uni --help`,
						'uni-cli-linux-amd64-install'
					)}

					{@render codesnippet(
						'Linux (ARM64): download, unpack, install, and verify:',
						`curl -LO ${shellDownloadBase}/uni_v0.26.3-uni_linux_arm64.tar.gz && tar xzf uni_v0.26.3-uni_linux_arm64.tar.gz && sudo install -m 0755 uni /usr/local/bin/uni && uni --help`,
						'uni-cli-linux-arm64-install'
					)}

					<p class="text-xs opacity-70">
						No admin rights? Install into a user-writable directory on your PATH instead, for
						example <code class="inline-code"
							>mkdir -p ~/.local/bin && install -m 0755 uni ~/.local/bin/uni</code
						>
						(make sure
						<code class="inline-code">~/.local/bin</code> is on your PATH).
					</p>

					{@render codesnippet(
						'Windows (PowerShell): unpack and run:',
						String.raw`Expand-Archive .\uni_v0.26.3-uni_windows_amd64.zip -DestinationPath .; .\uni.exe --help`,
						'uni-cli-windows-install'
					)}

					<p class="text-xs opacity-70">
						Verify each archive against <code class="inline-code">checksums.txt</code> above: on
						macOS run <code class="inline-code">shasum -a 256 &lt;file&gt;</code>, on Linux
						<code class="inline-code">sha256sum -c checksums.txt</code>, on Windows PowerShell
						<code class="inline-code">Get-FileHash &lt;file&gt; -Algorithm SHA256</code>. If macOS
						blocks the unpacked binary after a browser download, follow your organization's security
						policy to allow it.
					</p>
				{:else}
					<p class="text-sm mt-1" role="status">CLI download URL is not configured.</p>
				{/if}

				{@render codesnippet(
					'Windows (PowerShell): with uni installed, run:',
					windowsSetupCommand,
					'uni-cli-windows-setup-command'
				)}

				<p class="text-xs opacity-70">
					On Windows, optionally move <code class="inline-code">uni.exe</code> into a folder on your
					PATH so you can run it as <code class="inline-code">uni</code> (as shown in the commands
					below) without the <code class="inline-code">.\</code> prefix.
				</p>

				{@render codesnippet(
					'macOS/Linux: then run the following command:',
					setupCommand,
					'uni-cli-setup-command'
				)}
			</div>
		</section>

		<section class="flex items-center justify-center mb-12">
			<div class="max-w-full w-2xl paper h-full gap-2 py-8">
				<div
					class="p-3 rounded-full dark:bg-base-300 bg-base-200 w-fit justify-center items-center flex self-center"
				>
					<AntennaIcon class="@2xl/cli:size-10 size-6 text-primary translate-y-0.5" />
				</div>
				<h4 class="text-xl font-semibold text-center my-2">
					What does <code class="font-mono font-normal mx-2">uni setup</code> do?
				</h4>
				<ul class="list-disc font-light flex flex-col gap-2 px-4">
					<li>Detects Claude Code on your machine</li>
					<li>Authenticates and saves your default uni URL locally</li>
					<li>
						Installs uni bootstrap skills so AI clients know how to work with your org’s MCP catalog
					</li>
				</ul>
			</div>
		</section>

		<div class="divider"></div>

		<section class="mt-12 flex flex-col gap-4" id="uni-cli-commands">
			<h3 class="text-2xl font-bold">uni CLI Commands</h3>

			<div class="paper" id="uni-cli-command-setup">
				{@render commandPreview('uni setup')}
				<p>
					Use <code class="inline-code">uni setup</code> to authenticate with uni and install the uni
					skills into your AI clients.
				</p>
			</div>

			<div class="paper" id="uni-cli-command-mcp">
				{@render commandPreview('uni mcp')}
				<p class="mb-2">
					Use <code class="inline-code">uni mcp</code> to search uni for MCP servers and get the
					connection details for your AI client. We support Claude, Codex, and all clients that
					support
					<code class="inline-code">~/.agents</code>, including:
				</p>
				{@render supportedClients('size-6')}

				<ul class="list-disc font-light flex flex-col gap-2 @lg/cli:px-8 px-4 text-sm">
					<li>
						<p class="mb-2">
							Search uni for installable MCP servers from your AI client using the following skill:
						</p>
						<div class="mb-2">
							{@render slashCommandPreview(
								'/uni-search-mcp-servers',
								'Search uni for installable MCP servers. (user)'
							)}
						</div>
					</li>
				</ul>
			</div>

			<div class="paper" id="uni-cli-command-skills">
				{@render commandPreview('uni skills')}
				<p>
					Use <code class="inline-code">uni skills</code> to install and manage skills. We support
					Claude, Codex, and all clients that support <code class="inline-code">~/.agents</code>,
					including:
				</p>
				{@render supportedClients()}

				<ul class="list-disc font-light flex flex-col gap-2 @lg/cli:px-8 px-4 text-sm">
					<li>
						<p class="mb-2">
							Directly install skills to your AI clients using the following skills:
						</p>
						<div class="mb-2">
							{@render slashCommandPreview(
								'/uni-search-skills',
								'Search uni for installable skills. (user)'
							)}
						</div>
						{@render slashCommandPreview('/uni-install-skill', 'Install a skill from uni. (user)')}
					</li>
				</ul>
			</div>
		</section>
	</div>
</Layout>

{#snippet codesnippet(step: string, command: string, id: string)}
	<p class="text-sm">{step}</p>
	<div class="relative mt-0.5 mb-4 code-snippet">
		<pre class="pl-4 pr-22 py-2 m-0" {id}><code>{command}</code></pre>
		<div class="absolute top-1/2 right-2 -translate-y-1/2">
			<CopyButton
				text={command}
				id={`${id}-copy-button`}
				classes={{ button: 'flex shrink-0 gap-2 text-xs text-white hover:text-primary' }}
				showTextLeft
			/>
		</div>
	</div>
{/snippet}

{#snippet supportedClients(klass: string = 'size-8')}
	<ul class="flex flex-wrap gap-2 items-center">
		{#each marks as mark (mark.id)}
			<li class="tooltip bg-white dark:bg-base-200" data-tip={mark.alt}>
				{#if darkMode.isDark && mark.iconDark}
					<img src={mark.iconDark} alt={mark.alt} class={klass} />
				{:else}
					<img src={mark.icon} alt={mark.alt} class={klass} />
				{/if}
			</li>
		{/each}
	</ul>
{/snippet}

{#snippet commandPreview(command: string)}
	<div class="command-preview">
		<pre data-prefix="$" class="m-0"><code>{command}</code></pre>
	</div>
{/snippet}

{#snippet slashCommandPreview(command: string, description: string)}
	<div class="command-preview">
		<pre class="m-0"><code
				class="command-line-split flex w-full flex-col gap-1 whitespace-normal px-6 @lg/cli:flex-row @lg/cli:items-baseline @lg/cli:justify-between @lg/cli:gap-4"
				><span class="shrink-0">{command}</span><span
					class="w-full opacity-70 @lg/cli:w-auto @lg/cli:text-right">{description}</span
				></code
			></pre>
	</div>
{/snippet}

{#snippet mockupCode()}
	<div class="mockup-code w-full max-w-2xl">
		<pre></pre>
		<pre data-prefix="$"><code>uni setup</code></pre>
		<pre data-prefix=">"><code
				><CheckIcon class="text-success size-4 inline" /> Connected Claude Code</code
			></pre>
		<pre></pre>
		<pre data-prefix="$"><code>uni mcp search</code></pre>
		<pre data-prefix=">"><code
				><CheckIcon class="text-success size-4 inline" /> 24 MCP servers available</code
			></pre>
		<pre></pre>
		<pre data-prefix="$"><code>uni skills search</code></pre>
		<pre data-prefix=">"><code
				><CheckIcon class="text-success size-4 inline" /> 56 skills available</code
			></pre>

		<pre></pre>
	</div>
{/snippet}

<svelte:head>
	<title>uni CLI | Install</title>
</svelte:head>

<style lang="postcss">
	.mockup-code pre {
		margin-top: 0;
		margin-bottom: 0;
		border-radius: 0;
		background-color: var(--tw-prose-pre-bg);
		color: var(--tw-prose-pre-code);
	}

	.code-snippet pre {
		background-color: var(--tw-prose-pre-bg);
		color: var(--tw-prose-pre-code);
	}

	.mockup-code pre[data-prefix] {
		display: grid;
		grid-template-columns: 2rem minmax(0, 1fr);
		column-gap: 0.5rem;
	}

	.mockup-code pre[data-prefix]::before {
		grid-column: 1;
		grid-row: 1;
		align-self: start;
		display: block;
		width: 2rem;
	}

	.mockup-code pre[data-prefix] > code {
		grid-column: 2;
		grid-row: 1;
		min-width: 0;
		white-space: pre-wrap;
	}

	.inline-code {
		background-color: var(--color-base-200);
		color: var(--color-base-content);
		padding: 0.25rem 0.5rem;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);

		:global(.dark) & {
			background-color: var(--color-base-300);
		}
	}

	:global(.mockup-code pre code .text-success *) {
		color: var(--color-success) !important;
	}

	:global(.command-preview pre) {
		padding-top: 0.25rem;
		padding-bottom: 0.25rem;

		&[data-prefix]::before {
			content: attr(data-prefix);
			display: inline-block;
			width: calc(0.25rem * 8);
			text-align: right;
			opacity: 50%;
			margin-right: 0.5rem;
		}
	}
</style>
