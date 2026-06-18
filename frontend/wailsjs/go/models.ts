export namespace agent {
	
	export class Agent {
	    id: string;
	    name: string;
	    slug: string;
	    description?: string;
	    system_prompt?: string;
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
	
	    static createFrom(source: any = {}) {
	        return new Agent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.slug = source["slug"];
	        this.description = source["description"];
	        this.system_prompt = source["system_prompt"];
	        this.model_config = source["model_config"];
	        this.tool_bindings = source["tool_bindings"];
	        this.memory_config = source["memory_config"];
	        this.max_tokens = source["max_tokens"];
	        this.temperature = source["temperature"];
	        this.top_p = source["top_p"];
	        this.presence_penalty = source["presence_penalty"];
	        this.frequency_penalty = source["frequency_penalty"];
	        this.personality = source["personality"];
	        this.timeout_ms = source["timeout_ms"];
	        this.max_retries = source["max_retries"];
	        this.metadata = source["metadata"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class Session {
	    id: string;
	    agent_id: string;
	    title?: string;
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
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.agent_id = source["agent_id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.context_size = source["context_size"];
	        this.max_context_size = source["max_context_size"];
	        this.message_count = source["message_count"];
	        this.total_tokens_in = source["total_tokens_in"];
	        this.total_tokens_out = source["total_tokens_out"];
	        this.total_cost = source["total_cost"];
	        this.metadata = source["metadata"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.paused_at = source["paused_at"];
	        this.completed_at = source["completed_at"];
	    }
	}
	export class TurnResult {
	    message: string;
	    usage: model.TokenUsage;
	    model: string;
	    provider: string;
	    duration_ms: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TurnResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.usage = this.convertValues(source["usage"], model.TokenUsage);
	        this.model = source["model"];
	        this.provider = source["provider"];
	        this.duration_ms = source["duration_ms"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class agentParams {
	    max_tokens: number;
	    temperature: number;
	    top_p: number;
	    timeout_ms: number;
	    max_retries: number;
	    tool_ids: string[];
	
	    static createFrom(source: any = {}) {
	        return new agentParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.max_tokens = source["max_tokens"];
	        this.temperature = source["temperature"];
	        this.top_p = source["top_p"];
	        this.timeout_ms = source["timeout_ms"];
	        this.max_retries = source["max_retries"];
	        this.tool_ids = source["tool_ids"];
	    }
	}
	export class listAgentsFilter {
	    limit: number;
	    offset: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new listAgentsFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	        this.name = source["name"];
	    }
	}
	export class listSessionsFilter {
	    agent_id: string;
	    status: string;
	    limit: number;
	    offset: number;
	
	    static createFrom(source: any = {}) {
	        return new listSessionsFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.agent_id = source["agent_id"];
	        this.status = source["status"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	    }
	}
	export class providerInfo {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new providerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}

}

export namespace model {
	
	export class ModelInfo {
	    id: string;
	    name: string;
	    provider: string;
	    tags?: string[];
	    context_length: number;
	    max_output?: number;
	    cost_per_input_token?: number;
	    cost_per_output_token?: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.tags = source["tags"];
	        this.context_length = source["context_length"];
	        this.max_output = source["max_output"];
	        this.cost_per_input_token = source["cost_per_input_token"];
	        this.cost_per_output_token = source["cost_per_output_token"];
	    }
	}
	export class TokenUsage {
	    input_tokens: number;
	    output_tokens: number;
	    total_tokens: number;
	
	    static createFrom(source: any = {}) {
	        return new TokenUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.total_tokens = source["total_tokens"];
	    }
	}

}

