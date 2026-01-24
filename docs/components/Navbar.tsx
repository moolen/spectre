import React, { useState, useEffect } from 'react';
import { Ghost, Menu, X } from 'lucide-react';
import { NAV_LINKS } from '../constants';

const Navbar = () => {
    const [scrolled, setScrolled] = useState(false);
    const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

    useEffect(() => {
        const handleScroll = () => {
            setScrolled(window.scrollY > 20);
        };
        window.addEventListener('scroll', handleScroll);
        return () => window.removeEventListener('scroll', handleScroll);
    }, []);

    return (
        <nav className={`fixed w-full z-50 transition-all duration-300 border-b ${
            scrolled || mobileMenuOpen 
            ? 'bg-brand-bg/80 backdrop-blur-md border-brand-border py-3' 
            : 'bg-transparent border-transparent py-5'
        }`}>
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                <div className="flex justify-between items-center">
                    {/* Logo */}
                    <div className="flex items-center gap-2">
                        <div className="w-8 h-8 bg-gradient-to-br from-brand-indigo to-brand-primary rounded-lg flex items-center justify-center text-white shadow-lg shadow-brand-primary/20">
                            <Ghost className="w-5 h-5" />
                        </div>
                        <span className="font-bold text-xl text-white tracking-tight">Spectre</span>
                    </div>

                    {/* Desktop Links */}
                    <div className="hidden md:flex items-center gap-8">
                        {NAV_LINKS.map(link => (
                            <a 
                                key={link.name} 
                                href={link.href} 
                                className="text-sm font-medium text-slate-300 hover:text-white transition-colors"
                            >
                                {link.name}
                            </a>
                        ))}
                    </div>

                    {/* CTA & Mobile Toggle */}
                    <div className="flex items-center gap-4">
                        <a 
                            href="https://github.com" 
                            target="_blank" 
                            rel="noreferrer"
                            className="hidden md:flex items-center gap-2 px-4 py-2 bg-brand-surface hover:bg-brand-muted text-white rounded-lg text-sm font-medium transition-colors border border-brand-border"
                        >
                            <svg viewBox="0 0 24 24" className="w-4 h-4 fill-current" aria-hidden="true"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"></path></svg>
                            Star on GitHub
                        </a>
                        <button 
                            className="md:hidden text-white"
                            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                        >
                            {mobileMenuOpen ? <X /> : <Menu />}
                        </button>
                    </div>
                </div>
            </div>

            {/* Mobile Menu */}
            {mobileMenuOpen && (
                <div className="md:hidden bg-brand-bg border-t border-brand-border absolute w-full px-4 py-6 flex flex-col gap-4 shadow-2xl">
                     {NAV_LINKS.map(link => (
                        <a 
                            key={link.name} 
                            href={link.href} 
                            className="text-lg font-medium text-slate-300 hover:text-white"
                            onClick={() => setMobileMenuOpen(false)}
                        >
                            {link.name}
                        </a>
                    ))}
                    <a 
                        href="https://github.com" 
                        target="_blank" 
                        rel="noreferrer"
                        className="flex items-center justify-center gap-2 px-4 py-3 bg-white text-brand-bg rounded-lg text-lg font-bold"
                    >
                         Star on GitHub
                    </a>
                </div>
            )}
        </nav>
    );
};

export default Navbar;