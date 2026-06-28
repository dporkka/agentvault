export namespace contract {
	
	export class Source {
	    id: string;
	    path: string;
	    title: string;
	    excerpt?: string;
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.title = source["title"];
	        this.excerpt = source["excerpt"];
	    }
	}
	export class Answer {
	    answer: string;
	    sources: Source[];
	    confidence: string;
	    caveats?: string[];
	    missingInfo?: string;
	    suggestedActions?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Answer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.answer = source["answer"];
	        this.sources = this.convertValues(source["sources"], Source);
	        this.confidence = source["confidence"];
	        this.caveats = source["caveats"];
	        this.missingInfo = source["missingInfo"];
	        this.suggestedActions = source["suggestedActions"];
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
	export class NoteDetail {
	    id: string;
	    title: string;
	    path: string;
	    type: string;
	    project: string;
	    status: string;
	    tags: string[];
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new NoteDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.path = source["path"];
	        this.type = source["type"];
	        this.project = source["project"];
	        this.status = source["status"];
	        this.tags = source["tags"];
	        this.content = source["content"];
	    }
	}
	export class SearchResult {
	    id: string;
	    title: string;
	    path: string;
	    type: string;
	    project: string;
	    status: string;
	    tags: string[];
	    snippet: string;
	    score: number;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.path = source["path"];
	        this.type = source["type"];
	        this.project = source["project"];
	        this.status = source["status"];
	        this.tags = source["tags"];
	        this.snippet = source["snippet"];
	        this.score = source["score"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	
	export class VaultStatus {
	    path: string;
	    isVault: boolean;
	    noteCount: number;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new VaultStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.isVault = source["isVault"];
	        this.noteCount = source["noteCount"];
	        this.version = source["version"];
	    }
	}

}

export namespace main {
	
	export class AIStatus {
	    enabled: boolean;
	    provider: string;
	    model: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new AIStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.error = source["error"];
	    }
	}
	export class IndexingStatus {
	    isIndexing: boolean;
	    noteCount: number;
	
	    static createFrom(source: any = {}) {
	        return new IndexingStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isIndexing = source["isIndexing"];
	        this.noteCount = source["noteCount"];
	    }
	}

}

