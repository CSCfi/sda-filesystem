export namespace filesystem {
	
	export class Project {
	    name: string;
	    repository: string;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.repository = source["repository"];
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

