export function activeScenario(scope:string):string|null {
  if(typeof window==='undefined')return null;
  const supplied=new URL(window.location.href).searchParams.get('scenario_run_id');
  try {return supplied??window.sessionStorage.getItem('settlegraph.scenario:'+scope);}catch{return supplied;}
}
export function rememberScenario(scope:string,id:string){try{window.sessionStorage.setItem('settlegraph.scenario:'+scope,id);}catch{/* Navigation links retain the run context. */}}
