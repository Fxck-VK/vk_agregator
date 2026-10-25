/** Device-only public slugs. Never accepts prompts, account identities or artifact URLs. */
export function deviceBookmarks(key:string){
  const event=`epifera:bookmarks:${key}`;
  function read():string[]{const data:unknown=JSON.parse(localStorage.getItem(key)??"[]");return Array.isArray(data)?data.filter((id):id is string=>typeof id==="string"&&/^[a-z0-9-]{1,100}$/.test(id)).slice(0,100):[];}
  return {
    isSaved(id:string){try{return read().includes(id);}catch{return false;}},
    toggle(id:string){try{if(!/^[a-z0-9-]{1,100}$/.test(id))throw Error("Invalid public ID");const before=read(),saved=!before.includes(id);localStorage.setItem(key,JSON.stringify(saved?[...before.filter(value=>value!==id).slice(-99),id]:before.filter(value=>value!==id)));window.dispatchEvent(new Event(event));return {ok:true,saved} as const;}catch{return {ok:false} as const;}},
    subscribe(listener:()=>void){window.addEventListener("storage",listener);window.addEventListener(event,listener);return()=>{window.removeEventListener("storage",listener);window.removeEventListener(event,listener);};},
  };
}
