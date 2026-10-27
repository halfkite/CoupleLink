import router from '@system.router';
import storage from '@system.storage';
import { locales } from './locales';
const routes = ['home','interaction','chat','location','pet','anniversary','status','settings'];
export function createPage(index) {
  return {
    private: { words: locales.zh_CN, locale: 'zh_CN', accent: '#E58BB6', index: index,
      count: 8, confirmed: false, feedback: '', candidate: '', saveError: false },
    onInit() {
      var self = this;
      storage.get({key:'couplelink.preview.preferences', success:function(raw) {
        if (!raw) return;
        try {
          var config = JSON.parse(raw);
          self.locale = config.locale === 'en' ? 'en' : 'zh_CN';
          self.words = locales[self.locale];
          self.accent = config.accent === '#A89AF5' ? '#A89AF5' : '#E58BB6';
        } catch(e) { self.saveError = true; }
      }, fail:function(){ self.saveError = true; }});
    },
    prev() { router.replace({uri:'/pages/'+routes[(index+7)%8]}); },
    next() { router.replace({uri:'/pages/'+routes[(index+1)%8]}); },
    choose() { this.confirmed = true; this.feedback = ''; },
    preview() { this.confirmed = false; this.feedback = this.words['band.interaction.preview']; },
    cancel() { this.confirmed = false; },
    feed() { this.feedback = this.words['band.pet.done']; },
    reply() { this.feedback = this.words['band.interaction.preview']; },
    language() {
      this.locale = this.locale === 'en' ? 'zh_CN' : 'en';
      this.words = locales[this.locale]; this.persist();
    },
    theme() { this.accent = this.accent === '#E58BB6' ? '#A89AF5' : '#E58BB6'; this.persist(); },
    persist() {
      var self=this;
      storage.set({ key:'couplelink.preview.preferences', value:JSON.stringify({locale:this.locale,accent:this.accent}),
        success:function(){self.saveError=false;}, fail:function(){self.saveError=true;} });
    }
  };
}

