import {test,expect} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {spawn} from 'node:child_process';

test('web owner and native Android reviewer share one versioned case',async({page})=>{
  test.setTimeout(1800000);
  const root=path.resolve(process.cwd(),'..');
  const artifacts=path.resolve(root,process.env.CROSS_CLIENT_ARTIFACTS??'verification/cross-client');
  fs.mkdirSync(artifacts,{recursive:true});
  await page.goto('/login');await page.getByLabel('Display name').fill('Web owner');
  await page.getByRole('button',{name:'Enter workspace'}).click();await expect(page).toHaveURL(/dashboard/);
  const session=await (await page.request.get('/api/session')).json();
  await page.goto('/simulation');
  const creation=page.waitForResponse(r=>r.url().endsWith('/v1/demo/scenarios/delayed_settlement/runs')&&r.request().method()==='POST');
  await page.getByRole('button',{name:'Start selected scenario'}).click();
  const created=await creation;expect(created.status()).toBe(201);const run=await created.json();
  for(let i=0;i<2;i++){
    const advance=page.waitForResponse(r=>r.url().endsWith('/'+run.id+'/advance'));
    await page.getByRole('button',{name:'Advance step'}).click();expect((await advance).status()).toBe(200);
  }
  let record!:{id:string;version:number;assigned_to:string};
  await expect.poll(async()=>{
    const r=await page.request.get('/api/sg/v1/cases?scenario_run_id='+run.id);record=(await r.json()).items[0];return Boolean(record);
  },{timeout:30000}).toBeTruthy();
  await page.goto('/cases/'+record.id);
  const reviewer='22222222-2222-4222-8222-222222222223';
  await page.getByLabel('Assignee',{exact:true}).selectOption(reviewer);
  const assign=page.waitForResponse(r=>r.url().endsWith('/'+record.id+'/assign'));
  await page.getByRole('button',{name:'Reassign Case'}).click();expect((await assign).status()).toBe(200);
  await page.getByRole('button',{name:'Investigate evidence'}).click();
  await expect(page.locator('.ai-result').first()).toContainText('COMPLETED',{timeout:30000});
  const ai=await (await page.request.get('/api/sg/v1/cases/'+record.id+'/investigations')).json();
  record=await (await page.request.get('/api/sg/v1/cases/'+record.id)).json();
  const handoff=path.join(artifacts,'handoff.json');
  fs.writeFileSync(handoff,JSON.stringify({scenario_run_id:run.id,case_id:record.id,reviewer_id:reviewer,investigation_id:ai.items[0].id,evidence_snapshot_hash:ai.items[0].evidence_snapshot_hash},null,2));
  const child=spawn('python3',[path.join(root,'mobile_app/scripts/run_live_android.py'),'--device',process.env.ANDROID_DEVICE??'emulator-5554','--api-url',process.env.SETTLEGRAPH_API_URL??'http://127.0.0.1:8080','--device-api-url',process.env.ANDROID_API_URL??'http://10.0.2.2:8080','--handoff',handoff,'--coordination-dir',artifacts,'--artifacts',path.join(artifacts,'native')],{cwd:root,env:process.env});
  const log=fs.createWriteStream(path.join(artifacts,'native.log'));child.stdout.pipe(log);child.stderr.pipe(log);
  let exited=false;const finished=new Promise<number|null>((resolve,reject)=>{child.on('error',reject);child.on('exit',code=>{exited=true;resolve(code);});});
  try {
    await expect.poll(()=>{
      if(exited)throw new Error('Native runner exited before settlement handoff; inspect native.log');
      return fs.existsSync(path.join(artifacts,'settlement-requested.json'));
    },{timeout:1500000,intervals:[1000,2000]}).toBeTruthy();
    await page.goto('/simulation?scenario_run_id='+run.id);
    await expect(page.getByText('Step 2 of 3',{exact:true})).toBeVisible();
    const release=page.waitForResponse(r=>r.url().endsWith('/'+run.id+'/advance'));
    await page.getByRole('button',{name:'Advance step'}).click();expect((await release).status()).toBe(200);
    await expect.poll(async()=> (await (await page.request.get('/api/sg/v1/cases/'+record.id)).json()).status,{timeout:30000}).toBe('EVIDENCE_UPDATED');
    fs.writeFileSync(path.join(artifacts,'settlement-released.json'),JSON.stringify({case_id:record.id,source:'web-browser'}));
    expect(await finished).toBe(0);
    const native=JSON.parse(fs.readFileSync(path.join(artifacts,'native/trace.json'),'utf8'));
    expect(native.case_id).toBe(record.id);expect(native.mobile_user_id).toBe(reviewer);expect(native.web_user_id).toBe(session.user.id);expect(native.web_user_id).not.toBe(native.mobile_user_id);
    await page.goto('/cases/'+record.id);await expect(page.locator('.detail-status')).toContainText('RESOLVED');
    await expect(page.locator('.timeline')).toContainText('demo-reviewer');
    const final=await (await page.request.get('/api/sg/v1/cases/'+record.id)).json();expect(final.version).toBe(native.final_version);
    await page.screenshot({path:path.join(artifacts,'web-completed.png'),fullPage:true});
    fs.writeFileSync(path.join(artifacts,'trace.json'),JSON.stringify({...native,evidence_snapshot_hash:ai.items[0].evidence_snapshot_hash,web_verified_version:final.version,settlement_released_by:'web-browser'},null,2));
  } finally {if(!exited)child.kill('SIGTERM');log.end();}
});
