export namespace main {
	
	export class Source {
	    path: string;
	    title: string;
	    excerpt: string;
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.title = source["title"];
	        this.excerpt = source["excerpt"];
	    }
	}
	export class Answer {
	    answer: string;
	    sources: Source[];
	    confidence: string;
	    caveats: string[];
	    missingInfo: string;
	    suggestedActions: string[];
	
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
	export class Note {
	    id: string;
	    title: string;
	    path: string;
	    type: string;
	    project: string;
	    status: string;
	    tags: string[];
	    body: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Note(source);
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
	        this.body = source["body"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class SearchResult {
	    id: string;
	    title: string;
	    path: string;
	    type: string;
	    project: string;
	    tags: string[];
	    snippet: string;
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
	        this.tags = source["tags"];
	        this.snippet = source["snippet"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	
	export class VaultStatus {
	    path: string;
	    isOpen: boolean;
	    noteCount: number;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new VaultStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.isOpen = source["isOpen"];
	        this.noteCount = source["noteCount"];
	        this.version = source["version"];
	    }
	}

}

