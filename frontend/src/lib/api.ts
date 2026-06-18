import type {
	Agent,
	AgentParams,
	ListAgentsFilter,
	Session,
	ListSessionsFilter,
	TurnResult,
	ProviderInfo,
	ModelInfo,
} from "./types";

declare global {
	interface Window {
		go: {
			main: {
				App: {
					GetVersion(): Promise<string>;
					GetStatus(): Promise<string>;
					CreateAgent(
						name: string,
						systemPrompt: string,
						personality: string,
						params: AgentParams,
					): Promise<Agent>;
					GetAgent(id: string): Promise<Agent>;
					ListAgents(filter: ListAgentsFilter): Promise<Agent[]>;
					UpdateAgent(agent: Agent): Promise<void>;
					DeleteAgent(id: string): Promise<void>;
					CreateSession(agentID: string): Promise<Session>;
					GetSession(id: string): Promise<Session>;
					ListSessions(filter: ListSessionsFilter): Promise<Session[]>;
					DeleteSession(id: string): Promise<void>;
					PauseSession(id: string): Promise<void>;
					ResumeSession(id: string): Promise<void>;
					SendMessage(sessionID: string, message: string): Promise<TurnResult>;
					ListProviders(): Promise<ProviderInfo[]>;
					ListModels(providerID: string): Promise<ModelInfo[]>;
					GetAppSetting(key: string): Promise<string>;
					SetAppSetting(key: string, value: string): Promise<void>;
				};
			};
		};
	}
}

function app(): Window["go"]["main"]["App"] {
	return window.go.main.App;
}

export const api = {
	getVersion: () => app().GetVersion(),
	getStatus: () => app().GetStatus(),

	createAgent: (name: string, systemPrompt: string, personality: string, params: AgentParams = {}) =>
		app().CreateAgent(name, systemPrompt, personality, params),
	getAgent: (id: string) => app().GetAgent(id),
	listAgents: (filter: ListAgentsFilter = {}) => app().ListAgents(filter),
	updateAgent: (agent: Agent) => app().UpdateAgent(agent),
	deleteAgent: (id: string) => app().DeleteAgent(id),

	createSession: (agentID: string) => app().CreateSession(agentID),
	getSession: (id: string) => app().GetSession(id),
	listSessions: (filter: ListSessionsFilter = {}) => app().ListSessions(filter),
	deleteSession: (id: string) => app().DeleteSession(id),
	pauseSession: (id: string) => app().PauseSession(id),
	resumeSession: (id: string) => app().ResumeSession(id),

	sendMessage: (sessionID: string, message: string) => app().SendMessage(sessionID, message),

	listProviders: () => app().ListProviders(),
	listModels: (providerID: string) => app().ListModels(providerID),

	getSetting: (key: string) => app().GetAppSetting(key),
	setSetting: (key: string, value: string) => app().SetAppSetting(key, value),
};
