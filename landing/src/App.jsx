import './index.css';

import Navigation from './components/Navigation';
import Hero from './components/Hero';
import StatsBar from './components/StatsBar';
import Offerings from './components/Offerings';
import Features from './components/Features';
import UseCases from './components/UseCases';
import Architecture from './components/Architecture';
import GammaPresentation from './components/GammaPresentation';
import OpenSource from './components/OpenSource';
import Waitlist from './components/Waitlist';
import Footer from './components/Footer';

export default function App() {
  return (
    <div
      style={{
        background: '#05080f',
        minHeight: '100vh',
        cursor: 'default',
        scrollBehavior: 'smooth',
      }}
    >
      <Navigation />

      <main>
        <section id="home">
          <Hero />
        </section>

        <StatsBar />

        <section id="features">
          <Offerings />
          <Features />
        </section>

        <section id="use-cases">
          <UseCases />
        </section>

        <section id="architecture">
          <Architecture />
        </section>

        <GammaPresentation />

        <OpenSource />

        <Waitlist />
      </main>

      <Footer />
    </div>
  );
}
