import React, { useState, useEffect, useRef } from 'react';
import { HEADLINES } from '../constants';
import { ArrowRight, Activity, GitBranch, Terminal } from 'lucide-react';

const Hero = () => {
    const [index, setIndex] = useState(0);
    const [isVisible, setIsVisible] = useState(true);
    const canvasRef = useRef<HTMLCanvasElement>(null);

    // Canvas Star Animation
    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;

        const ctx = canvas.getContext('2d');
        if (!ctx) return;

        let width = canvas.width = canvas.offsetWidth;
        let height = canvas.height = canvas.offsetHeight;
        
        const handleResize = () => {
             if (canvas) {
                width = canvas.width = canvas.offsetWidth;
                height = canvas.height = canvas.offsetHeight;
             }
        };
        window.addEventListener('resize', handleResize);

        // Star settings
        const STAR_COUNT = 400; 
        const SPEED = 0.7; // Movement speed (Z-axis units per frame)
        
        interface Star {
            x: number;
            y: number;
            z: number;
            baseSize: number;
            twinkleSpeed: number;
            twinklePhase: number;
        }

        const stars: Star[] = [];
        
        // Initialize stars
        for(let i=0; i<STAR_COUNT; i++) {
             stars.push({
                 x: (Math.random() - 0.5) * width * 1.5,
                 y: (Math.random() - 0.5) * height * 1.5,
                 z: Math.random() * 1000,
                 baseSize: Math.random() * 1.5 + 0.5,
                 twinkleSpeed: Math.random() * 0.05 + 0.01,
                 twinklePhase: Math.random() * Math.PI * 2
             });
        }

        let animationFrameId: number;

        const render = () => {
            ctx.clearRect(0, 0, width, height);
            
            const cx = width / 2;
            const cy = height / 2;
            const fov = 400; // Field of view

            stars.forEach(star => {
                // Move star towards camera (decrease Z)
                star.z -= SPEED;
                star.twinklePhase += star.twinkleSpeed;

                // Reset star if it passes the camera or goes out of bounds
                if (star.z <= 1) {
                    star.z = 1000;
                    star.x = (Math.random() - 0.5) * width * 1.5;
                    star.y = (Math.random() - 0.5) * height * 1.5;
                }

                // 3D to 2D Projection
                const scale = fov / star.z;
                const x2d = cx + star.x * scale;
                const y2d = cy + star.y * scale;

                // Opacity calculations
                // 1. Twinkle effect
                const twinkle = Math.sin(star.twinklePhase) * 0.5 + 0.5; 
                // 2. Distance fade (fade out when far away)
                const distanceFade = Math.min(1, (1000 - star.z) / 500);
                // 3. Near fade (fade out when very close to avoid giant circles)
                const nearFade = Math.min(1, (star.z) / 100);

                const alpha = (0.3 + (twinkle * 0.7)) * distanceFade * nearFade;

                if (alpha > 0.01) {
                    const size = star.baseSize * scale;
                    
                    ctx.beginPath();
                    ctx.fillStyle = `rgba(255, 255, 255, ${alpha})`;
                    ctx.arc(x2d, y2d, size, 0, Math.PI * 2);
                    ctx.fill();
                }
            });

            animationFrameId = requestAnimationFrame(render);
        };

        render();

        return () => {
            window.removeEventListener('resize', handleResize);
            cancelAnimationFrame(animationFrameId);
        };
    }, []);

    // Headline Rotation Logic
    useEffect(() => {
        const interval = setInterval(() => {
            // Start exit animation
            setIsVisible(false);

            // Wait for exit animation to complete (500ms match CSS), then update text and enter
            setTimeout(() => {
                setIndex((prevIndex) => (prevIndex + 1) % HEADLINES.length);
                setIsVisible(true);
            }, 500); 

        }, 4500); // Total cycle duration

        return () => clearInterval(interval);
    }, []);

    const currentHeadline = HEADLINES[index];

    return (
        <section className="relative pt-32 pb-20 lg:pt-48 lg:pb-32 overflow-hidden min-h-[90vh] flex flex-col justify-center">
            {/* Background Layer Group */}
            <div className="absolute inset-0 overflow-hidden pointer-events-none select-none">
                
                {/* 1. Ambient Color Blobs */}
                <div className="absolute top-0 left-1/4 w-96 h-96 bg-brand-primary/20 rounded-full mix-blend-screen filter blur-[100px] opacity-30 animate-blob"></div>
                <div className="absolute top-20 right-1/4 w-96 h-96 bg-brand-indigo/20 rounded-full mix-blend-screen filter blur-[100px] opacity-30 animate-blob animation-delay-2000"></div>
                <div className="absolute -bottom-32 left-1/3 w-96 h-96 bg-brand-violet/20 rounded-full mix-blend-screen filter blur-[100px] opacity-30 animate-blob animation-delay-4000"></div>

                {/* 2. Canvas Starfield */}
                <canvas 
                    ref={canvasRef}
                    className="absolute inset-0 w-full h-full z-0"
                />
            </div>

            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 relative text-center z-10">
                {/* Badge */}
                <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-brand-surface border border-brand-border/50 backdrop-blur-sm text-sm text-slate-300 mb-8 hover:bg-brand-muted transition-colors cursor-default shadow-sm shadow-brand-primary/10">
                    <span className="relative flex h-2 w-2">
                      <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-brand-primary opacity-75"></span>
                      <span className="relative inline-flex rounded-full h-2 w-2 bg-brand-primary"></span>
                    </span>
                    Spectre v1.0 Public Beta
                </div>

                {/* Main Headline */}
                <h1 className="text-4xl md:text-6xl lg:text-7xl font-bold tracking-tight text-white mb-4 leading-tight max-w-6xl mx-auto">
                    <div className="mb-2 md:mb-0">80% of incidents are caused</div>
                    
                    <div className="flex flex-wrap justify-center gap-x-3 md:grid md:grid-cols-12 md:gap-x-4 min-h-[3em] md:min-h-[1.5em]">
                        <div className="md:col-span-5 md:text-right">by</div>
                        <div className="md:col-span-7 md:text-left">
                             <span 
                                className={`inline-block bg-gradient-to-r from-brand-indigo via-brand-primary to-brand-violet bg-clip-text text-transparent drop-shadow-[0_2px_10px_rgba(168,85,247,0.3)] box-decoration-clone transition-all duration-500 ease-in-out pb-2 ${
                                    isVisible 
                                    ? 'opacity-100 translate-y-0 blur-0' 
                                    : 'opacity-0 translate-y-4 blur-sm'
                                }`}
                            >
                                {currentHeadline.head}
                            </span>
                        </div>
                    </div>
                </h1>

                {/* Subheadline */}
                <div className="min-h-[4rem] flex items-start justify-center mb-10">
                    <p 
                        className={`text-lg md:text-xl text-slate-400 max-w-2xl mx-auto transition-all duration-500 ease-in-out transform ${
                            isVisible 
                            ? 'opacity-100 translate-y-0' 
                            : 'opacity-0 -translate-y-4'
                        }`}
                    >
                        {currentHeadline.sub}
                    </p>
                </div>

                {/* Buttons */}
                <div className="flex flex-col sm:flex-row gap-4 justify-center items-center">
                    <button className="group relative px-8 py-4 bg-white text-brand-bg rounded-lg font-semibold text-lg hover:bg-slate-200 transition-all active:scale-95 w-full sm:w-auto overflow-hidden">
                        <span className="relative z-10 flex items-center justify-center gap-2">
                            Get Started Free <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />
                        </span>
                    </button>
                    <button className="px-8 py-4 bg-brand-muted/50 border border-brand-border text-white rounded-lg font-semibold text-lg hover:bg-brand-muted transition-all active:scale-95 w-full sm:w-auto backdrop-blur-sm">
                        View Demo
                    </button>
                </div>

                {/* Tech Stack Hints */}
                <div className="mt-20 pt-10 border-t border-brand-border/30 flex flex-wrap justify-center gap-8 md:gap-16">
                    <div className="flex items-center gap-2 text-slate-300">
                        <Terminal className="w-5 h-5 text-brand-primary" />
                        <span className="font-semibold">Kubernetes Native</span>
                    </div>
                    <div className="flex items-center gap-2 text-slate-300">
                        <Activity className="w-5 h-5 text-brand-primary" />
                        <span className="font-semibold">Real-time Events</span>
                    </div>
                    <div className="flex items-center gap-2 text-slate-300">
                        <GitBranch className="w-5 h-5 text-brand-primary" />
                        <span className="font-semibold">GitOps Aware</span>
                    </div>
                </div>
            </div>
        </section>
    );
};

export default Hero;