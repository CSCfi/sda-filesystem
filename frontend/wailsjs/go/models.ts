export namespace airlock {
	
	export class UploadSet {
	    bucket: string;
	    files: string[];
	    objects: string[];
	    exists: boolean[];
	
	    static createFrom(source: any = {}) {
	        return new UploadSet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bucket = source["bucket"];
	        this.files = source["files"];
	        this.objects = source["objects"];
	        this.exists = source["exists"];
	    }
	}

}

export namespace main {
	
	export class Log {
	    loglevel: string;
	    timestamp: string;
	    message: string[];
	
	    static createFrom(source: any = {}) {
	        return new Log(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loglevel = source["loglevel"];
	        this.timestamp = source["timestamp"];
	        this.message = source["message"];
	    }
	}

}

