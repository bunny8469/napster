export namespace __ {
	
	export class SongInfo {
	    file_name?: string;
	    artist_name?: string;
	    peer_addresses?: string[];
	    created_at?: string;
	    duration?: string;
	
	    static createFrom(source: any = {}) {
	        return new SongInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_name = source["file_name"];
	        this.artist_name = source["artist_name"];
	        this.peer_addresses = source["peer_addresses"];
	        this.created_at = source["created_at"];
	        this.duration = source["duration"];
	    }
	}

}

export namespace client {
	
	export class TorrentMetadata {
	    file_name: string;
	    file_size: number;
	    chunk_size: number;
	    checksum: string;
	    chunk_checksums: Record<number, string>;
	    peers: string[];
	    artist_name: string;
	    created_at: string;
	    duration: number;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new TorrentMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_name = source["file_name"];
	        this.file_size = source["file_size"];
	        this.chunk_size = source["chunk_size"];
	        this.checksum = source["checksum"];
	        this.chunk_checksums = source["chunk_checksums"];
	        this.peers = source["peers"];
	        this.artist_name = source["artist_name"];
	        this.created_at = source["created_at"];
	        this.duration = source["duration"];
	        this.status = source["status"];
	    }
	}
	export class TorrentInfo {
	    Metadata: TorrentMetadata;
	    Progress: number;
	    Status: string;
	
	    static createFrom(source: any = {}) {
	        return new TorrentInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Metadata = this.convertValues(source["Metadata"], TorrentMetadata);
	        this.Progress = source["Progress"];
	        this.Status = source["Status"];
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

