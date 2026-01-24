import React from 'react';
import {
    Puzzle,
    Settings,
    Ambulance,
    GitCommit,
    Network,
    Activity,
    Route,
    X,
    Check
} from 'lucide-react';

const Features = () => {
    const steps = [
        {
            icon: GitCommit,
            title: "Track Change",
            description: "Spectre records meaningful changes across your cluster — not just deploys, but what changed and where."
        },
        {
            icon: Network,
            title: "Model Relationships",
            description: "Resources aren't isolated. Spectre understands ownership, dependencies, and runtime relationships."
        },
        {
            icon: Activity,
            title: "Detect Anomalies",
            description: "Metrics, logs, and signals from your existing stack indicate when behavior shifts."
        },
        {
            icon: Route,
            title: "Trace Causality",
            description: "Spectre links anomalies back to the changes that most likely caused them."
        }
    ];

    const beforeItems = [
        "\"What changed?\"",
        "\"Who deployed?\"",
        "\"Is this related or coincidence?\"",
        "Pager goes off → Slack explodes → context is missing"
    ];

    const afterItems = [
        "Ranked list of likely causes",
        "Clear change timelines",
        "Impacted services and blast radius",
        "Faster, calmer incident calls"
    ];

    return (
        <section id="features" className="py-24 bg-brand-bg relative">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">

                {/* WHAT SPECTRE DOES */}
                <div className="mb-32">
                    <div className="flex items-center gap-3 mb-6">
                        <Puzzle className="w-8 h-8 text-brand-primary" />
                        <span className="text-sm font-semibold text-brand-primary uppercase tracking-wider">What Spectre Does</span>
                    </div>
                    <h2 className="text-3xl md:text-5xl font-bold text-white mb-6">
                        Spectre connects changes to impact.{' '}
                        <span className="text-transparent bg-clip-text bg-gradient-to-r from-brand-indigo via-brand-primary to-brand-violet">
                            Automatically.
                        </span>
                    </h2>
                    <div className="max-w-3xl">
                        <p className="text-lg text-slate-400 mb-6 leading-relaxed">
                            Spectre continuously observes your Kubernetes environment and builds a living model of:
                        </p>
                        <ul className="text-lg text-slate-300 space-y-3 mb-8">
                            <li className="flex items-start gap-3">
                                <span className="w-1.5 h-1.5 rounded-full bg-brand-primary mt-2.5 flex-shrink-0" />
                                Resources and their relationships
                            </li>
                            <li className="flex items-start gap-3">
                                <span className="w-1.5 h-1.5 rounded-full bg-brand-primary mt-2.5 flex-shrink-0" />
                                Configuration and deployment changes over time
                            </li>
                            <li className="flex items-start gap-3">
                                <span className="w-1.5 h-1.5 rounded-full bg-brand-primary mt-2.5 flex-shrink-0" />
                                Anomalies, failures, and behavioral shifts
                            </li>
                        </ul>
                        <p className="text-lg text-slate-400 mb-6 leading-relaxed">
                            When an incident happens, Spectre correlates change events with observed anomalies to surface the most likely causal paths.
                        </p>
                        <p className="text-xl font-semibold text-white">
                            No guesswork. No log archaeology.
                        </p>
                    </div>
                </div>

                {/* HOW IT WORKS */}
                <div className="mb-32">
                    <div className="flex items-center gap-3 mb-6">
                        <Settings className="w-8 h-8 text-brand-primary" />
                        <span className="text-sm font-semibold text-brand-primary uppercase tracking-wider">How It Works</span>
                    </div>
                    <h2 className="text-3xl md:text-5xl font-bold text-white mb-12">
                        Built for how Kubernetes{' '}
                        <span className="text-transparent bg-clip-text bg-gradient-to-r from-brand-indigo via-brand-primary to-brand-violet">
                            actually behaves
                        </span>
                    </h2>

                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                        {steps.map((step, i) => (
                            <div key={i} className="relative">
                                <div className="group p-6 bg-brand-surface/50 border border-brand-border rounded-2xl hover:border-brand-primary/50 transition-all duration-300 hover:bg-brand-surface h-full">
                                    <div className="absolute inset-0 bg-gradient-to-br from-brand-primary/5 to-brand-indigo/5 opacity-0 group-hover:opacity-100 transition-opacity rounded-2xl" />
                                    <div className="flex items-center gap-3 mb-4">
                                        <span className="text-sm font-bold text-brand-primary">Step {i + 1}</span>
                                    </div>
                                    <div className="mb-4 inline-flex items-center justify-center w-12 h-12 rounded-lg bg-brand-muted border border-brand-border group-hover:border-brand-primary/50 transition-all">
                                        <step.icon className="w-6 h-6 text-brand-primary" />
                                    </div>
                                    <h3 className="text-xl font-bold text-white mb-3 relative z-10">{step.title}</h3>
                                    <p className="text-slate-400 leading-relaxed relative z-10">{step.description}</p>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>

                {/* INCIDENT RESPONSE */}
                <div>
                    <div className="flex items-center gap-3 mb-6">
                        <Ambulance className="w-8 h-8 text-brand-primary" />
                        <span className="text-sm font-semibold text-brand-primary uppercase tracking-wider">Incident Response</span>
                    </div>
                    <h2 className="text-3xl md:text-5xl font-bold text-white mb-12">
                        From firefighting to{' '}
                        <span className="text-transparent bg-clip-text bg-gradient-to-r from-brand-indigo via-brand-primary to-brand-violet">
                            focused response
                        </span>
                    </h2>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                        {/* Before */}
                        <div className="p-8 bg-brand-surface/30 border border-red-500/20 rounded-2xl">
                            <div className="flex items-center gap-3 mb-6">
                                <div className="w-10 h-10 rounded-full bg-red-500/10 flex items-center justify-center">
                                    <X className="w-5 h-5 text-red-400" />
                                </div>
                                <h3 className="text-xl font-bold text-white">Before Spectre</h3>
                            </div>
                            <ul className="space-y-4">
                                {beforeItems.map((item, i) => (
                                    <li key={i} className="flex items-start gap-3 text-slate-400">
                                        <span className="w-1.5 h-1.5 rounded-full bg-red-400/50 mt-2.5 flex-shrink-0" />
                                        {item}
                                    </li>
                                ))}
                            </ul>
                        </div>

                        {/* After */}
                        <div className="p-8 bg-brand-surface/50 border border-brand-primary/30 rounded-2xl">
                            <div className="flex items-center gap-3 mb-6">
                                <div className="w-10 h-10 rounded-full bg-brand-primary/10 flex items-center justify-center">
                                    <Check className="w-5 h-5 text-brand-primary" />
                                </div>
                                <h3 className="text-xl font-bold text-white">With Spectre</h3>
                            </div>
                            <ul className="space-y-4">
                                {afterItems.map((item, i) => (
                                    <li key={i} className="flex items-start gap-3 text-slate-300">
                                        <span className="w-1.5 h-1.5 rounded-full bg-brand-primary mt-2.5 flex-shrink-0" />
                                        {item}
                                    </li>
                                ))}
                            </ul>
                        </div>
                    </div>
                </div>

            </div>
        </section>
    );
};

export default Features;