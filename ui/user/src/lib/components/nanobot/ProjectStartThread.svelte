<script lang="ts">
	import Thread from '$lib/components/nanobot/Thread.svelte';
	import type { ChatSession } from '$lib/services/nanobot/chat/index.svelte';
	import { MessageCircle, Sparkles } from 'lucide-svelte';

	interface Props {
		agentId: string;
		projectId: string;
		chat: ChatSession;
		browserBaseUrl?: string;
		browserAvailable?: boolean;
		browserViewerOpen?: boolean;
		onFileOpen?: (filename: string) => void;
		suppressEmptyState?: boolean;
		onThreadContentWidth?: (width: number) => void;
		classes?: {
			root?: string;
		};
	}

	let {
		chat,
		browserBaseUrl = '',
		browserAvailable = false,
		browserViewerOpen = $bindable(false),
		onFileOpen,
		suppressEmptyState,
		onThreadContentWidth,
		classes
	}: Props = $props();
</script>

<div class="flex h-full w-full">
	<div class="h-full min-w-0 flex-1">
		{#key chat.chatId}
			<Thread
				messages={chat.messages}
				prompts={chat.prompts}
				elicitations={chat.elicitations}
				agents={chat.agents}
				selectedAgentId={chat.selectedAgentId}
				onAgentChange={chat.selectAgent}
				onElicitationResult={chat.replyToElicitation}
				onSendMessage={chat.sendMessage}
				onFileUpload={chat.uploadFile}
				onCancel={chat.cancelMessage}
				cancelUpload={chat.cancelUpload}
				uploadingFiles={chat.uploadingFiles}
				uploadedFiles={chat.uploadedFiles}
				isLoading={chat.isLoading}
				isRestoring={chat.isRestoring}
				agent={chat.agent}
				onRefreshResources={() => {
					chat.refreshResources();
				}}
				{onFileOpen}
				{browserBaseUrl}
				{browserAvailable}
				bind:browserViewerOpen
				onReadResource={chat.readResource}
				{suppressEmptyState}
				onContentWidthChange={onThreadContentWidth}
				{classes}
			>
				{#snippet emptyStateContent()}
					<div class="flex flex-col items-center gap-4 px-5 pb-5 md:pb-0">
						<div class="flex flex-col items-center gap-1">
							<h1 class="w-full text-center text-xl font-semibold md:text-3xl">
								你想从哪里开始？
							</h1>
							<p class="text-base-content/50 md:text-md text-center text-sm font-light">
								选择一个入口，或直接开始对话。
							</p>
						</div>
						<div class="flex w-full flex-col items-center justify-center gap-4 md:flex-row">
							<button
								class="bg-base-200 dark:border-base-300 rounded-field hover:bg-base-300 col-span-1 h-full w-full p-4 text-left shadow-sm md:w-70"
								onclick={() => {
									chat?.sendMessage('我想设计一个 AI 工作流。请帮我开始。');
								}}
							>
								<Sparkles class="mb-4 size-5" />
								<h3 class="text-base font-semibold">创建工作流</h3>
								<p class="text-base-content/50 text-sm font-light">
									通过对话设计并执行一个智能体工作流
								</p>
							</button>
							<button
								class="bg-base-200 dark:border-base-300 rounded-field hover:bg-base-300 col-span-1 h-full w-full p-4 text-left shadow-sm md:w-70"
								onclick={() => {
									chat?.sendMessage(
										'帮我了解你能做什么。请说明你的能力，并建议几个我们可以尝试的方向。'
									);
								}}
							>
								<MessageCircle class="mb-4 size-5" />
								<h3 class="text-base font-semibold">先了解一下</h3>
								<p class="text-base-content/50 min-h-[2lh] text-sm font-light">
									了解这个智能体能做什么，然后再继续
								</p>
							</button>
						</div>
					</div>
				{/snippet}
			</Thread>
		{/key}
	</div>
</div>
