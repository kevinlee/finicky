import {describe,expect,it} from 'vitest';
import {openUrl} from './index';
describe('picker routing',()=>{
  it('returns a typed prompt action for defaults and explicit rules',()=>{
    const result=openUrl('https://example.com',null,null,{defaultBrowser:'prompt'});
    expect(result.browser).toMatchObject({action:'prompt',appType:'none',name:''});
    expect(result.browser.matchedRule).toBeUndefined();
    const matched=openUrl('https://example.com',null,null,{defaultBrowser:'Safari',handlers:[{match:'*',browser:'prompt'}]});
    expect(matched.browser).toMatchObject({action:'prompt',matchedRule:0});
  });
  it('preserves private/new window launches and rule-specific rewrites',()=>{
    const result=openUrl('https://example.com/old',null,null,{defaultBrowser:'Safari',handlers:[{match:'*',url:(url:URL)=>{url.pathname='/new';return url;},browser:{name:'Google Chrome',profileDirectory:'Profile 2',private:true,newWindow:true}}]});
    expect(result.browser).toMatchObject({url:'https://example.com/new',private:true,newWindow:true,profileDirectory:'Profile 2',matchedRule:0});
  });
  it('transforms before matching and after selecting a rule',()=>{
    const result=openUrl('https://old.example/start',null,null,{
      defaultBrowser:'Safari', options:{transformScript:"return url.replace('old.example','new.example');"},
      handlers:[{match:'https://new.example/*', ruleIndex:4, transformScript:"return url.replace('/start','/finish');",browser:{name:'com.google.Chrome',private:true}}]
    });
    expect(result.browser).toMatchObject({url:'https://new.example/finish',name:'com.google.Chrome',appType:'bundleId',private:true,matchedRule:4});
    expect(result.error).toBeUndefined();
  });
  it('reports invalid transformation results',()=>{
    const result=openUrl('https://example.com',null,null,{defaultBrowser:'Safari',options:{transformScript:'return undefined;'}});
    expect(result.warnings[0]).toContain('non-empty URL string');
    expect(result.error).toBeUndefined();
    expect(result.browser.url).toBe('https://example.com/');
  });
});
