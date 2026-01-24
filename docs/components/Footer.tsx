import React from 'react';
import { Ghost, Twitter, Github, Linkedin } from 'lucide-react';

const Footer = () => {
    return (
        <footer className="bg-brand-dark border-t border-brand-border pt-16 pb-8">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                <div className="grid grid-cols-1 md:grid-cols-4 gap-12 mb-12">
                    <div className="col-span-1 md:col-span-1">
                        <div className="flex items-center gap-2 mb-4">
                            <div className="w-8 h-8 bg-gradient-to-br from-brand-indigo to-brand-primary rounded-lg flex items-center justify-center text-white">
                                <Ghost className="w-5 h-5" />
                            </div>
                            <span className="font-bold text-xl text-white">Spectre</span>
                        </div>
                        <p className="text-slate-400 text-sm leading-relaxed">
                            Built by engineers who've debugged distributed systems at 3am.
                            <br />
                            Because "what changed?" shouldn't be a mystery.
                        </p>
                    </div>
                    
                    <div>
                        <h4 className="text-white font-semibold mb-4">Product</h4>
                        <ul className="space-y-2">
                            <li><a href="#" className="text-slate-400 hover:text-brand-primary text-sm transition-colors">Features</a></li>
                            <li><a href="#" className="text-slate-400 hover:text-brand-primary text-sm transition-colors">Integrations</a></li>
                            <li><a href="#" className="text-slate-400 hover:text-brand-primary text-sm transition-colors">Changelog</a></li>
                            <li><a href="#" className="text-slate-400 hover:text-brand-primary text-sm transition-colors">Docs</a></li>
                        </ul>
                    </div>
                    
                    <div>
                        <h4 className="text-white font-semibold mb-4">Community</h4>
                        <ul className="space-y-2">
                            <li><a href="#" className="text-slate-400 hover:text-brand-primary text-sm transition-colors">GitHub</a></li>
                            <li><a href="#" className="text-slate-400 hover:text-brand-primary text-sm transition-colors">Discord</a></li>
                            <li><a href="#" className="text-slate-400 hover:text-brand-primary text-sm transition-colors">Twitter</a></li>
                        </ul>
                    </div>

                    <div>
                        <h4 className="text-white font-semibold mb-4">Legal</h4>
                        <ul className="space-y-2">
                            <li><a href="#" className="text-slate-400 hover:text-brand-primary text-sm transition-colors">Privacy Policy</a></li>
                            <li><a href="#" className="text-slate-400 hover:text-brand-primary text-sm transition-colors">Terms of Service</a></li>
                        </ul>
                    </div>
                </div>
                
                <div className="border-t border-brand-border/30 pt-8 flex flex-col md:flex-row justify-between items-center gap-4">
                    <p className="text-slate-500 text-sm">© {new Date().getFullYear()} Spectre Observability. All rights reserved.</p>
                    <div className="flex gap-4">
                        <a href="#" className="text-slate-500 hover:text-white transition-colors"><Twitter className="w-5 h-5" /></a>
                        <a href="#" className="text-slate-500 hover:text-white transition-colors"><Github className="w-5 h-5" /></a>
                        <a href="#" className="text-slate-500 hover:text-white transition-colors"><Linkedin className="w-5 h-5" /></a>
                    </div>
                </div>
            </div>
        </footer>
    );
};

export default Footer;