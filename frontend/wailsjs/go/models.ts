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

