import React from 'react';
import Navbar from './components/Navbar';
import Hero from './components/Hero';
import Features from './components/Features';
import Footer from './components/Footer';

const App = () => {
  return (
    <div className="min-h-screen bg-brand-bg text-white selection:bg-brand-primary/30">
      <Navbar />
      <main>
        <Hero />
        <Features />
        
        {/* Simple integration banner */}
        <section className="py-20 border-y border-brand-border/50 bg-brand-surface/30">
             <div className="max-w-4xl mx-auto px-4 text-center">
                <h2 className="text-2xl md:text-3xl font-bold mb-8">Works with your favorite stacks</h2>
                <div className="flex flex-wrap justify-center gap-8 md:gap-12 opacity-60">
                    {/* Placeholder text for logos to avoid external image dependencies issues */}
                    <span className="text-xl font-bold text-slate-300">Prometheus</span>
                    <span className="text-xl font-bold text-slate-300">Grafana</span>
                    <span className="text-xl font-bold text-slate-300">ArgoCD</span>
                    <span className="text-xl font-bold text-slate-300">Flux</span>
                    <span className="text-xl font-bold text-slate-300">OpenTelemetry</span>
                </div>
             </div>
        </section>

        {/* CTA Section */}
        <section className="py-24 relative overflow-hidden">
            <div className="absolute inset-0 bg-gradient-to-b from-brand-bg to-brand-dark z-0"></div>
            <div className="max-w-4xl mx-auto px-4 relative z-10 text-center">
                <h2 className="text-3xl md:text-5xl font-bold mb-6">Stop guessing why production broke.</h2>
                <p className="text-slate-400 text-lg mb-10 max-w-2xl mx-auto">
                    Spectre gives you causal understanding of your Kubernetes systems — before, during, and after incidents.
                </p>
                <div className="flex flex-col sm:flex-row gap-4 justify-center">
                    <button className="px-8 py-4 bg-gradient-to-r from-brand-indigo via-brand-primary to-brand-violet text-white rounded-lg font-bold text-lg hover:shadow-lg hover:shadow-brand-primary/25 transition-all transform hover:-translate-y-1 border border-brand-primary/20">
                        Install with Helm
                    </button>
                    <button className="px-8 py-4 bg-brand-muted border border-brand-border text-white rounded-lg font-bold text-lg hover:bg-brand-surface transition-all">
                        Read the Docs
                    </button>
                </div>
            </div>
        </section>

      </main>
      <Footer />
    </div>
  );
};

export default App;