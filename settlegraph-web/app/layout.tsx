import type { Metadata } from 'next';
import './globals.css';
export const metadata:Metadata={title:'SettleGraph AI — Payment Intelligence',description:'Evidence-backed multi-provider financial reconciliation dashboard (demo environment)'};
export default function RootLayout({children}:{children:React.ReactNode}){return <html lang="en"><body>{children}</body></html>;}
