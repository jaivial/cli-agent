export namespace app {
	
	export class ContextUsage {
	    estimated_tokens: number;
	    context_window_tokens: number;
	    threshold_tokens: number;
	    percent_used: number;
	    percent_left: number;
	
	    static createFrom(source: any = {}) {
	        return new ContextUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.estimated_tokens = source["estimated_tokens"];
	        this.context_window_tokens = source["context_window_tokens"];
	        this.threshold_tokens = source["threshold_tokens"];
	        this.percent_used = source["percent_used"];
	        this.percent_left = source["percent_left"];
	    }
	}

}

export namespace main {
	
	export class DesktopChatMessage {
	    id: string;
	    role: string;
	    content: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new DesktopChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.created_at = this.convertValues(source["created_at"], null);
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
	export class DesktopRunStatus {
	    run_id: string;
	    status: string;
	    ready: boolean;
	    // Go type: time
	    ready_at: any;
	    api_key_configured: boolean;
	    mode: string;
	    has_tmux: boolean;
	    max_parallel_agents: number;
	    orchestrate_parallel: number;
	    model: string;
	    base_url: string;
	
	    static createFrom(source: any = {}) {
	        return new DesktopRunStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.run_id = source["run_id"];
	        this.status = source["status"];
	        this.ready = source["ready"];
	        this.ready_at = this.convertValues(source["ready_at"], null);
	        this.api_key_configured = source["api_key_configured"];
	        this.mode = source["mode"];
	        this.has_tmux = source["has_tmux"];
	        this.max_parallel_agents = source["max_parallel_agents"];
	        this.orchestrate_parallel = source["orchestrate_parallel"];
	        this.model = source["model"];
	        this.base_url = source["base_url"];
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
	export class DesktopSessionCard {
	    id: string;
	    root_id: string;
	    title: string;
	    // Go type: time
	    last_activity: any;
	    message_count: number;
	
	    static createFrom(source: any = {}) {
	        return new DesktopSessionCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.root_id = source["root_id"];
	        this.title = source["title"];
	        this.last_activity = this.convertValues(source["last_activity"], null);
	        this.message_count = source["message_count"];
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

