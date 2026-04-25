import './index.css';
import { useTheme } from './hooks/useTheme';

import Navigation from './components/Navigation';
import Hero from './components/Hero';
import StatsBar from './components/StatsBar';
import Positioning from './components/Positioning';
import Comparison from './components/Comparison';
import Features from './components/Features';
import Architecture from './components/Architecture';
import OpenSource from './components/OpenSource';
import Waitlist from './components/Waitlist';
import Footer from './components/Footer';

export default function App() {
  const { currentTheme } = useTheme();
  
  return (
    <div
      style={{
        background: currentTheme.bg.primary,
        minHeight: '100vh',
        cursor: 'default',
        scrollBehavior: 'smooth',
        transition: 'background-color 300ms ease-in-out',
      }}
    >
      <Navigation />

      <main>
        <section id="home">
          <Hero />
        </section>

        <StatsBar />

        <Positioning />

        <Comparison />

        <section id="architecture">
          <Architecture />
        </section>

        <section id="features">
          <Features />
        </section>

        <section id="open-source">
          <OpenSource />
        </section>

        <Waitlist />
      </main>

      <Footer />
    </div>
  );
}
