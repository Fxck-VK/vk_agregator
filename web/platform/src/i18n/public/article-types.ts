export type ArticleInline=string|{readonly text:string;readonly strong?:boolean;readonly href?:`/${string}`|`#${string}`};
export type ArticleBlock=
  | {readonly type:"paragraph";readonly content:readonly ArticleInline[]}
  | {readonly type:"list";readonly ordered?:boolean;readonly items:readonly {readonly content:readonly ArticleInline[];readonly example?:boolean}[]}
  | {readonly type:"image";readonly imageIndex:number};
export type ArticleRecord={
  readonly title:string;readonly lead:readonly ArticleBlock[];
  readonly sections:readonly {readonly id:string;readonly label:string;readonly title:string;readonly blocks:readonly ArticleBlock[]}[];
  readonly closing:readonly ArticleBlock[];
};
