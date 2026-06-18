export interface Agent {
	id: string;
	name: string;
	slug: string;
	description: string;
	system_prompt: string;
	model_config: string;
	tool_bindings: string;
	memory_config: string;
	max_tokens: number;
	temperature: number;
	top_p: number;
	presence_penalty: number;
	frequency_penalty: number;
	personality: string;
	timeout_ms: number;
	max_retries: number;
	metadata: string;
	created_at: string;
	updated_at: string;
}

export interface Session {
	id: string;
	agent_id: string;
	title: string;
	status: string;
	context_size: number;
	max_context_size: number;
	message_count: number;
	total_tokens_in: number;
	total_tokens_out: number;
	total_cost: number;
	metadata: string;
	created_at: string;
	updated_at: string;
	paused_at?: string;
	completed_at?: string;
}

export interface TurnResult {
	message: string;
	usage: TokenUsage;
	model: string;
	provider: string;
	duration_ms: number;
	error?: string;
}

export interface TokenUsage {
	input_tokens: number;
	output_tokens: number;
	total_tokens: number;
}

export interface ProviderInfo {
	id: string;
	name: string;
}

export interface ModelInfo {
	id: string;
	name: string;
	provider: string;
	tags: string[];
	context_length: number;
}

export interface AgentParams {
	max_tokens?: number;
	temperature?: number;
	top_p?: number;
	timeout_ms?: number;
	max_retries?: number;
	tool_ids?: string[];
}

export interface ListAgentsFilter {
	limit?: number;
	offset?: number;
	name?: string;
}

export interface ListSessionsFilter {
	agent_id?: string;
	status?: string;
	limit?: number;
	offset?: number;
}
