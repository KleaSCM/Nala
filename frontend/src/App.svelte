<script lang="ts">
	let sidebarCollapsed = $state(false);
	let currentView = $state("chat");
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
				<button class="nav-item glass-light" class:active={currentView === "memory"} onclick={() => (currentView = "memory")}>
					<span class="nav-icon">🧠</span>
					<span class="nav-label">Memory</span>
				</button>
				<button class="nav-item glass-light" class:active={currentView === "notes"} onclick={() => (currentView = "notes")}>
					<span class="nav-icon">📝</span>
					<span class="nav-label">Notes</span>
				</button>
				<button class="nav-item glass-light" class:active={currentView === "settings"} onclick={() => (currentView = "settings")}>
					<span class="nav-icon">⚙️</span>
					<span class="nav-label">Settings</span>
				</button>
			</nav>

			<div class="sidebar-footer glass-light">
				<div class="status-dot"></div>
				<span class="status-text">Ready</span>
			</div>
		{/if}
	</aside>

	<main class="main">
		<header class="header glass">
			<div class="header-title">{currentView.charAt(0).toUpperCase() + currentView.slice(1)}</div>
			<div class="header-actions">
				<button class="glass-button">New Chat</button>
			</div>
		</header>

		<section class="content">
			{#if currentView === "chat"}
				<div class="chat-container">
					<div class="messages">
						<div class="welcome glass">
							<div class="welcome-icon">🐱</div>
							<h1 class="welcome-title">Hello, I'm Nala</h1>
							<p class="welcome-subtitle">Your personal AI companion</p>
							<div class="suggestions">
								<button class="glass-button suggestion">What can you help me with?</button>
								<button class="glass-button suggestion">Set up my agents</button>
								<button class="glass-button suggestion">Tell me about memory</button>
							</div>
						</div>
					</div>

					<div class="input-area glass">
						<input
							type="text"
							class="glass-input chat-input"
							placeholder="Ask me anything..."
						/>
						<button class="glass-button send-btn">Send</button>
					</div>
				</div>
			{:else if currentView === "settings"}
				<div class="settings-container">
					<div class="settings-card glass">
						<h2>Model</h2>
						<div class="setting-row">
							<label for="provider-select">Provider</label>
							<select id="provider-select" class="glass-input">
								<option>Ollama</option>
								<option>OpenAI</option>
								<option>Anthropic</option>
							</select>
						</div>
						<div class="setting-row">
							<label for="default-model">Default Model</label>
							<input id="default-model" type="text" class="glass-input" value="llama3.2:3b" />
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
			{:else}
				<div class="placeholder glass">
					<span class="placeholder-icon">
						{#if currentView === "agents"}🤖{:else if currentView === "memory"}🧠{:else if currentView === "notes"}📝{/if}
					</span>
					<p>Coming soon</p>
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
		justify-content: center;
		align-items: center;
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

	.suggestions {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.suggestion {
		width: 100%;
		padding: 12px 20px;
		font-size: 14px;
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

	.placeholder {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 64px;
		border-radius: var(--radius-lg);
		color: var(--text-muted);
		gap: 12px;
	}

	.placeholder-icon {
		font-size: 48px;
	}
</style>
