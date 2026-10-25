import { cookies } from 'next/headers';
import { redirect } from 'next/navigation';
import { Shell } from '@/components/shell';
export default async function WorkspaceLayout({children}:{children:React.ReactNode}){
 const jar=await cookies();if(!jar.get('sg_token')?.value)redirect('/login');return <Shell>{children}</Shell>;
}
