<script lang="ts">
	import { onMount } from "svelte";
	import { api } from "./lib/api";
	import type { Agent, Session, TurnResult } from "./lib/types";

	let sidebarCollapsed = $state(false);
	let currentView = $state("chat");

	let agents = $state<Agent[]>([]);
	let sessions = $state<Session[]>([]);
	let currentSessionId = $state<string | null>(null);
	let messages = $state<{ role: string; content: string; model?: string }[]>([]);
	let chatInput = $state("");
	let sending = $state(false);
	let statusText = $state("Loading…");
	let statusOk = $state(false);
	let errorMsg = $state("");

	let providers = $state<{ id: string; name: string }[]>([]);
	let models = $state<{ id: string; name: string }[]>([]);
	let selectedProvider = $state("ollama");
	let selectedModel = $state("");
	let settingsLoaded = $state(false);

	onMount(async () => {
		try {
			const status = await api.getStatus();
			statusText = status;
			statusOk = true;
		} catch {
			statusText = "Offline";
		}
		loadAgents();
		loadSessions();
		loadProviders();
	});

	async function loadProviders() {
		try {
			providers = await api.listProviders();
			if (providers.length > 0) {
				selectedProvider = providers[0].id;
				loadModels(selectedProvider);
			}
		} catch (e: any) {
			console.error("Failed to load providers", e);
		}
		settingsLoaded = true;
	}

	async function loadModels(providerID: string) {
		try {
			const ms = await api.listModels(providerID);
			models = ms.map((m: any) => ({ id: m.id, name: m.id }));
			if (models.length > 0 && !selectedModel) {
				selectedModel = models[0].id;
			}
		} catch (e: any) {
			models = [];
		}
	}

	$effect(() => {
		if (selectedProvider && settingsLoaded) {
			selectedModel = "";
			loadModels(selectedProvider);
		}
	});

	async function loadAgents() {
		try {
			agents = await api.listAgents({});
			if (agents.length === 0) {
				const a = await api.createAgent("Default Assistant", "", "default", {});
				agents = [a];
			}
		} catch (e: any) {
			console.error("Failed to load agents", e);
		}
	}

	async function loadSessions() {
		try {
			sessions = await api.listSessions({});
		} catch (e: any) {
			console.error("Failed to load sessions", e);
		}
	}

	async function startNewChat() {
		if (agents.length === 0) return;
		try {
			const s = await api.createSession(agents[0].id);
			sessions = [s, ...sessions];
			currentSessionId = s.id;
			messages = [];
			currentView = "chat";
		} catch (e: any) {
			errorMsg = "Failed to create session";
		}
	}

	async function sendMessage() {
		const text = chatInput.trim();
		if (!text || sending) return;
		chatInput = "";
		sending = true;
		errorMsg = "";

		if (!currentSessionId) {
			if (agents.length === 0) {
				errorMsg = "No agent available";
				sending = false;
				return;
			}
			try {
				const s = await api.createSession(agents[0].id);
				sessions = [s, ...sessions];
				currentSessionId = s.id;
			} catch (e: any) {
				errorMsg = "Failed to create session";
				sending = false;
				return;
			}
		}

		messages = [...messages, { role: "user", content: text }];

		try {
			const result: TurnResult = await api.sendMessage(currentSessionId!, text);
			if (result.error) {
				messages = [...messages, { role: "assistant", content: `Error: ${result.error}` }];
			} else {
				messages = [...messages, { role: "assistant", content: result.message, model: result.model }];
			}
		} catch (e: any) {
			messages = [...messages, { role: "assistant", content: `Error: ${e.message || e}` }];
		} finally {
			sending = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === "Enter" && !e.shiftKey) {
			e.preventDefault();
			sendMessage();
		}
	}

	function selectSession(id: string) {
		currentSessionId = id;
		messages = [];
		currentView = "chat";
	}

	function createNewAgent() {
		const name = prompt("Agent name:", "New Agent");
		if (!name) return;
		api.createAgent(name, "", "default", {}).then(a => {
			agents = [...agents, a];
		}).catch((e: any) => {
			errorMsg = `Failed to create agent: ${e.message || e}`;
		});
	}

	function deleteAgent(id: string) {
		if (!confirm("Delete this agent?")) return;
		api.deleteAgent(id).then(() => {
			agents = agents.filter(a => a.id !== id);
		}).catch((e: any) => {
			errorMsg = `Failed to delete agent: ${e.message || e}`;
		});
	}
</script>

<div class="layout">
	<aside class="sidebar glass" class:collapsed={sidebarCollapsed}>
		<div class="sidebar-header">
			<div class="logo">
				<span class="logo-icon">🐱</span>
				{#if !sidebarCollapsed}<span class="logo-text">Nala</span>{/if}
			</div>
			<button class="toggle-btn glass-button" onclick={() => (sidebarCollapsed = !sidebarCollapsed)}>
				{sidebarCollapsed ? "☰" : "✕"}
			</button>
		</div>

		{#if !sidebarCollapsed}
			<nav class="nav">
				<button class="nav-item glass-light" class:active={currentView === "chat"} onclick={() => (currentView = "chat")}>
					<span class="nav-icon">💬</span>
					<span class="nav-label">Chat</span>
				</button>
				<button class="nav-item glass-light" class:active={currentView === "agents"} onclick={() => (currentView = "agents")}>
					<span class="nav-icon">🤖</span>
					<span class="nav-label">Agents</span>
				</button>
				<button class="nav-item glass-light" class:active={currentView === "settings"} onclick={() => (currentView = "settings")}>
					<span class="nav-icon">⚙️</span>
					<span class="nav-label">Settings</span>
				</button>
			</nav>

			<div class="session-list">
				<div class="session-list-header">Sessions</div>
				{#each sessions as s}
					<button
						class="session-item glass-light"
						class:active={s.id === currentSessionId}
						onclick={() => selectSession(s.id)}
					>
						<span class="session-title">{s.title || "New Chat"}</span>
					</button>
				{/each}
			</div>

			<div class="sidebar-footer glass-light">
				<div class="status-dot" class:online={statusOk}></div>
				<span class="status-text">{statusText}</span>
			</div>
		{/if}
	</aside>

	<main class="main">
		<header class="header glass">
			<div class="header-title">{currentView === "chat" ? "Chat" : currentView.charAt(0).toUpperCase() + currentView.slice(1)}</div>
			<div class="header-actions">
				<button class="glass-button" onclick={startNewChat}>New Chat</button>
			</div>
		</header>

		<section class="content">
			{#if currentView === "chat"}
				<div class="chat-container">
					<div class="messages" class:centered={messages.length === 0}>
						{#if messages.length === 0}
							<div class="welcome glass">
								<div class="welcome-icon">🐱</div>
								<h1 class="welcome-title">Hello, I'm Nala</h1>
								<p class="welcome-subtitle">Your personal AI companion</p>
							</div>
						{:else}
							{#each messages as msg}
								<div class="message" class:user={msg.role === "user"} class:assistant={msg.role === "assistant"}>
									<div class="message-sender">{msg.role === "user" ? "You" : "Nala"}</div>
									<div class="message-content">{msg.content}</div>
									{#if msg.model}
										<div class="message-model">{msg.model}</div>
									{/if}
								</div>
							{/each}
						{/if}
					</div>

					{#if errorMsg}
						<div class="error-bar">{errorMsg}</div>
					{/if}

					<div class="input-area glass">
						<input
							type="text"
							class="glass-input chat-input"
							placeholder="Ask me anything..."
							bind:value={chatInput}
							onkeydown={handleKeydown}
							disabled={sending}
						/>
						<button class="glass-button send-btn" onclick={sendMessage} disabled={sending}>
							{sending ? "..." : "Send"}
						</button>
					</div>
				</div>
			{:else if currentView === "settings"}
				<div class="settings-container">
					<div class="settings-card glass">
						<h2>Provider</h2>
						<div class="setting-row">
							<label for="provider-select">Provider</label>
							<select id="provider-select" class="glass-input nala-select" bind:value={selectedProvider}>
								{#each providers as p}
									<option value={p.id}>{p.name}</option>
								{/each}
							</select>
						</div>
						<div class="setting-row">
							<label for="model-select">Model</label>
							<select id="model-select" class="glass-input nala-select" bind:value={selectedModel}>
								{#each models as m}
									<option value={m.id}>{m.id}</option>
								{/each}
							</select>
						</div>
					</div>
					<div class="settings-card glass">
						<h2>Memory</h2>
						<div class="setting-row">
							<label for="auto-extract">Auto-extract facts</label>
							<input id="auto-extract" type="checkbox" checked />
						</div>
					</div>
				</div>
			{:else if currentView === "agents"}
				<div class="agents-container">
					<div class="agents-header">
						<button class="glass-button" onclick={createNewAgent}>+ New Agent</button>
					</div>
					{#each agents as a}
						<div class="agent-card glass">
							<div class="agent-info">
								<div class="agent-name">{a.name}</div>
								<div class="agent-slug">@{a.slug}</div>
								{#if a.personality && a.personality !== "default"}
									<div class="agent-tag">{a.personality}</div>
								{/if}
							</div>
							<button class="glass-button agent-delete" onclick={() => deleteAgent(a.id)}>✕</button>
						</div>
					{/each}
				</div>
			{/if}
		</section>
	</main>
</div>

<style>
	.layout {
		display: flex;
		height: 100%;
	}

	.sidebar {
		width: var(--sidebar-width);
		display: flex;
		flex-direction: column;
		padding: 16px;
		gap: 8px;
		transition: width 0.25s ease;
		border-right: 1px solid var(--glass-border);
	}

	.sidebar.collapsed {
		width: 64px;
	}

	.sidebar-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 16px;
	}

	.logo {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.logo-icon {
		font-size: 28px;
	}

	.logo-text {
		font-size: 20px;
		font-weight: 700;
		background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
		-webkit-background-clip: text;
		-webkit-text-fill-color: transparent;
		background-clip: text;
	}

	.toggle-btn {
		font-size: 16px;
		padding: 4px 10px;
	}

	.nav {
		display: flex;
		flex-direction: column;
		gap: 4px;
		flex: 1;
	}

	.nav-item {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
		border-radius: var(--radius-sm);
		cursor: pointer;
		font-size: 14px;
		color: var(--text-secondary);
		transition: background 0.2s, color 0.2s;
	}

	.nav-item:hover {
		background: rgba(255, 255, 255, 0.08);
		color: var(--text-primary);
	}

	.nav-item.active {
		background: rgba(167, 139, 250, 0.12);
		color: var(--accent-primary);
	}

	.nav-icon {
		font-size: 18px;
		width: 24px;
		text-align: center;
	}

	.nav-label {
		white-space: nowrap;
	}

	.sidebar-footer {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 12px;
		border-radius: var(--radius-sm);
		margin-top: auto;
	}

	.status-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--accent-tertiary);
		box-shadow: 0 0 8px rgba(52, 211, 153, 0.5);
	}

	.status-text {
		font-size: 12px;
		color: var(--text-muted);
	}

	.main {
		flex: 1;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0 24px;
		height: var(--header-height);
		border-bottom: 1px solid var(--glass-border);
		flex-shrink: 0;
	}

	.header-title {
		font-size: 15px;
		font-weight: 600;
		color: var(--text-secondary);
		text-transform: capitalize;
	}

	.header-actions {
		display: flex;
		gap: 8px;
	}

	.content {
		flex: 1;
		overflow-y: auto;
		padding: 24px;
	}

	.chat-container {
		display: flex;
		flex-direction: column;
		height: 100%;
	}

	.messages {
		flex: 1;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		padding: 16px 0;
	}

	.welcome {
		text-align: center;
		padding: 48px;
		border-radius: var(--radius-lg);
		max-width: 480px;
	}

	.welcome-icon {
		font-size: 64px;
		margin-bottom: 16px;
	}

	.welcome-title {
		font-size: 28px;
		font-weight: 700;
		margin-bottom: 8px;
	}

	.welcome-subtitle {
		color: var(--text-secondary);
		margin-bottom: 32px;
	}

	.input-area {
		display: flex;
		gap: 8px;
		padding: 16px;
		border-radius: var(--radius-md);
		margin-top: 16px;
	}

	.chat-input {
		flex: 1;
	}

	.send-btn {
		padding: 10px 24px;
		font-weight: 600;
		white-space: nowrap;
	}

	.session-list {
		display: flex;
		flex-direction: column;
		gap: 2px;
		margin-top: 8px;
		flex: 1;
		overflow-y: auto;
	}

	.session-list-header {
		font-size: 11px;
		font-weight: 600;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.5px;
		padding: 8px 12px 4px;
	}

	.session-item {
		padding: 8px 12px;
		border-radius: var(--radius-sm);
		cursor: pointer;
		font-size: 13px;
		color: var(--text-secondary);
		transition: background 0.2s, color 0.2s;
		text-align: left;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.session-item:hover {
		background: rgba(255, 255, 255, 0.08);
		color: var(--text-primary);
	}

	.session-item.active {
		background: rgba(167, 139, 250, 0.12);
		color: var(--accent-primary);
	}

	.session-title {
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.messages.centered {
		display: flex;
		flex-direction: column;
		justify-content: center;
		align-items: center;
	}

	.message {
		padding: 12px 16px;
		margin-bottom: 8px;
		border-radius: var(--radius-md);
		max-width: 80%;
	}

	.message.user {
		background: rgba(167, 139, 250, 0.15);
		align-self: flex-end;
	}

	.message.assistant {
		background: rgba(255, 255, 255, 0.05);
		align-self: flex-start;
	}

	.message-sender {
		font-size: 11px;
		font-weight: 600;
		color: var(--text-muted);
		margin-bottom: 4px;
	}

	.message-content {
		font-size: 14px;
		line-height: 1.5;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.message-model {
		font-size: 10px;
		color: var(--text-muted);
		margin-top: 4px;
		opacity: 0.6;
	}

	.error-bar {
		background: rgba(239, 68, 68, 0.15);
		color: #fca5a5;
		padding: 8px 16px;
		font-size: 13px;
		border-radius: var(--radius-sm);
		margin-bottom: 8px;
	}

	.status-dot.online {
		box-shadow: 0 0 8px rgba(52, 211, 153, 0.5);
	}

	.settings-container {
		display: flex;
		flex-direction: column;
		gap: 16px;
		max-width: 600px;
	}

	.settings-card {
		padding: 24px;
		border-radius: var(--radius-md);
	}

	.settings-card h2 {
		font-size: 16px;
		font-weight: 600;
		margin-bottom: 16px;
		color: var(--accent-primary);
	}

	.setting-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 8px 0;
	}

	.setting-row label {
		font-size: 14px;
		color: var(--text-secondary);
	}

	.nala-select {
		-webkit-appearance: none !important;
		appearance: none !important;
		background-color: #0f0a1a !important;
		color: #f1f5f9 !important;
		border: 1px solid rgba(167, 139, 250, 0.3) !important;
		border-radius: 8px !important;
		padding: 10px 32px 10px 14px !important;
		font-size: 14px !important;
		cursor: pointer !important;
		background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' fill='%23a78bfa' viewBox='0 0 12 12'%3E%3Cpath d='M6 8L1 3h10z'/%3E%3C/svg%3E") !important;
		background-repeat: no-repeat !important;
		background-position: right 12px center !important;
	}

	.nala-select option {
		background-color: #0f0a1a !important;
		color: #f1f5f9 !important;
	}

	.nala-select:focus {
		border-color: #a78bfa !important;
		box-shadow: 0 0 0 3px rgba(167, 139, 250, 0.2) !important;
	}

	.agents-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
		max-width: 600px;
	}

	.agents-header {
		margin-bottom: 8px;
	}

	.agent-card {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 16px;
		border-radius: var(--radius-md);
	}

	.agent-info {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.agent-name {
		font-size: 15px;
		font-weight: 600;
	}

	.agent-slug {
		font-size: 12px;
		color: var(--text-muted);
	}

	.agent-tag {
		font-size: 11px;
		background: rgba(167, 139, 250, 0.15);
		color: var(--accent-primary);
		padding: 2px 8px;
		border-radius: 99px;
		display: inline-block;
		width: fit-content;
		margin-top: 2px;
	}

	.agent-delete {
		padding: 4px 10px;
		font-size: 12px;
	}
</style>
