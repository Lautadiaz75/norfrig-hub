export namespace main {
	
	export class PhotoItem {
	    id: number;
	    path: string;
	    filename: string;
	    root: string;
	    sku_primary: string;
	    size_bytes: number;
	    mtime: string;
	    thumb_url: string;
	    full_url: string;
	
	    static createFrom(source: any = {}) {
	        return new PhotoItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.filename = source["filename"];
	        this.root = source["root"];
	        this.sku_primary = source["sku_primary"];
	        this.size_bytes = source["size_bytes"];
	        this.mtime = source["mtime"];
	        this.thumb_url = source["thumb_url"];
	        this.full_url = source["full_url"];
	    }
	}
	export class SearchResult {
	    results: PhotoItem[];
	    total: number;
	    took_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.results = this.convertValues(source["results"], PhotoItem);
	        this.total = source["total"];
	        this.took_ms = source["took_ms"];
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
	export class StatsResult {
	    total_photos: number;
	    indexed_roots: string[];
	
	    static createFrom(source: any = {}) {
	        return new StatsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_photos = source["total_photos"];
	        this.indexed_roots = source["indexed_roots"];
	    }
	}

}

